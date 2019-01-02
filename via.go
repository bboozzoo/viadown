// The MIT License (MIT)
//
// Copyright (c) 2016 Maciek Borzecki <maciek.borzecki@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var (
	ErrUpstreamFailed    = errors.New("upstream request failed")
	ErrUpstreamBadStatus = errors.New("upstream returned unexpected status")
	ErrInternal          = errors.New("internal error")
)

type ViaDownloadServer struct {
	Mirrors       *Mirrors
	Cache         *Cache
	ClientTimeout time.Duration
	Router        *mux.Router
}

func loggingMiddleware(next http.Handler) http.Handler {
	buf := bytes.Buffer{}
	lh := handlers.LoggingHandler(&buf, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lh.ServeHTTP(w, r)
		log.Info(strings.TrimSpace(buf.String()))
		buf.Reset()
	})
}

func NewViaDownloadServer(mirrors *Mirrors, cache *Cache, clientTimeout time.Duration) *ViaDownloadServer {
	vs := &ViaDownloadServer{
		Mirrors:       mirrors,
		Cache:         cache,
		ClientTimeout: clientTimeout,
	}
	r := mux.NewRouter()
	r.HandleFunc("/_viadown/count", vs.countHandler).Methods(http.MethodGet)
	r.HandleFunc("/_viadown/stats", vs.statsHandler).Methods(http.MethodGet)
	r.HandleFunc("/_viadown/data", vs.dataDeleteHandler).Methods(http.MethodDelete)
	r.PathPrefix("/").Methods(http.MethodGet).HandlerFunc(vs.maybeCachedHandler)
	r.Use(loggingMiddleware)
	vs.Router = r

	return vs
}

func (v *ViaDownloadServer) returnError(w http.ResponseWriter, status int, err error) {
	type apiError struct {
		Error string
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.Encode(apiError{Error: err.Error()})
}

func (v *ViaDownloadServer) returnOk(w http.ResponseWriter, what interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.Encode(what)
}

func (v *ViaDownloadServer) statsHandler(w http.ResponseWriter, r *http.Request) {
	log.Infof("stats handler")
	v.returnOk(w, v.Cache.Stats())
}

func (v *ViaDownloadServer) countHandler(w http.ResponseWriter, r *http.Request) {
	log.Infof("count handler")
	count, err := v.Cache.Count()
	if err != nil {
		v.returnError(w, http.StatusInternalServerError, err)
		return
	}
	v.returnOk(w, count)
}

func (v *ViaDownloadServer) dataDeleteHandler(w http.ResponseWriter, r *http.Request) {
	log.Infof("cache purge handler")
	if err := r.ParseForm(); err != nil {
		v.returnError(w, http.StatusBadRequest, errors.New("malformed request"))
		return
	}
	s := r.FormValue("older-than-days")
	if s == "" {
		v.returnError(w, http.StatusBadRequest, errors.New("older-than-days not provided"))
		return
	}
	olderThanDays, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		v.returnError(w, http.StatusBadRequest, errors.New("older-than-days is not an integer"))
		return
	}
	removed, err := v.Cache.Purge(PurgeSelector{
		OlderThan: time.Now().Add(-time.Duration(olderThanDays) * 24 * time.Hour),
	})
	if err != nil {
		v.returnError(w, http.StatusInternalServerError, err)
		return
	}
	type removedInfo struct {
		Removed uint64
	}
	v.returnOk(w, removedInfo{Removed: removed})
}

func (v *ViaDownloadServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	v.Router.ServeHTTP(w, r)
}

func (v *ViaDownloadServer) maybeCachedHandler(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path

	if since, err := http.ParseTime(r.Header.Get("If-Modified-Since")); err == nil {
		log.Debugf("has modified since: %v, poke upstream first", since)
	} else {
		// no modified since header, try to get from cache
		found, err := doFromCache(upath, w, r, v.Cache)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		if found {
			return
		}
	}

	client := http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: v.ClientTimeout,
			}).Dial,
			TLSHandshakeTimeout:   v.ClientTimeout,
			ResponseHeaderTimeout: v.ClientTimeout,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	for _, mirror := range v.Mirrors.List {
		url := buildURL(mirror, upath)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			log.Errorf("failed to prepare request: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		// copy some headers from the original request
		copyHeaders(req.Header, r.Header,
			[]string{"Accept", "If-Modified-Since"})

		err = doFromUpstream(upath, &client, req, w, v.Cache)
		switch {
		case err == ErrInternal:
			w.WriteHeader(http.StatusInternalServerError)
			return
		case err == ErrUpstreamBadStatus:
			return
		case err == nil:
			return
		}
	}
}

func doFromCache(name string, w http.ResponseWriter, r *http.Request, cache *Cache) (bool, error) {
	cachedr, sz, err := cache.Get(name)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		log.Errorf("cache get failed: %v", err)
		return false, errors.New("cache access failed")
	}

	log.Debugf("getting from cache, size: %v", sz)
	defer cachedr.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, name, time.Now(), cachedr)

	return true, nil
}

func doFromUpstream(name string, client *http.Client, req *http.Request,
	w http.ResponseWriter, cache *Cache) error {

	rsp, err := client.Do(req)
	if err != nil {
		return ErrUpstreamFailed
	}
	log.Debugf("got response: %v", rsp)
	defer rsp.Body.Close()

	if rsp.StatusCode != 200 {
		log.Errorf("got status %v from upstream %s",
			rsp.StatusCode, req.URL)
		// TODO be smart, return ErrMirrorTryAnother for 404 requests
		// possibly
		copyHeaders(w.Header(), rsp.Header,
			[]string{"Content-Type", "Content-Length",
				"ETag", "Last-Modified",
				"Date"})
		w.WriteHeader(rsp.StatusCode)
		// got non 200 status, just forward
		io.Copy(w, rsp.Body)
		return ErrUpstreamBadStatus
	}

	out, err := cache.Put(name)
	if err != nil {
		return ErrInternal
	}
	defer out.Commit()

	// setup TeeReader so that the data makes to the disk while it's also
	// sent to the original requester
	tr := io.TeeReader(rsp.Body, out)

	// copy over headers from upstream response
	copyHeaders(w.Header(), rsp.Header,
		[]string{"Content-Type", "Content-Length",
			"ETag", "Last-Modified",
			"Date"})
	// let the client know we're good
	w.WriteHeader(http.StatusOK)

	log.Infof("downloading %v from %s to cache", name, req.URL)
	// send over the data
	if _, err := io.Copy(w, tr); err != nil {
		// we've already sent a status header, we're just streaming data
		// now, if that fails, discard any data cached so far
		log.Errorf("copy failed: %v, discarding cache entry", err)
		if err := out.Abort(); err != nil {
			log.Errorf("failed to discard cache entry: %v", err)
		}
	}
	log.Debugf("upstream download finished")
	return nil
}

func copyHeaders(to http.Header, from http.Header, which []string) {
	for _, hdr := range which {
		hv := from.Get(hdr)
		if hv != "" {
			to.Set(hdr, hv)
		}
	}
}

func buildURL(base, path string) string {
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}

	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	return base + path
}
