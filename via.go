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
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
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
}

func (v *ViaDownloadServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	upath := r.URL.Path
	log.Debugf("URL path: %v", upath)

	log.Debugf("client headers:")
	for h, v := range r.Header {
		log.Debugf("  %v: %v", h, v)
	}

	if since, err := http.ParseTime(r.Header.Get("If-Modified-Since")); err == nil {
		log.Debugf("has modified since: %v, poke upstream first", since)
	} else {
		// no modified since header, try to get from cache
		found, err := doFromCache(upath, w, v.Cache)
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

func doFromCache(name string, w http.ResponseWriter, cache *Cache) (bool, error) {
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

	w.Header().Set("Content-Length", strconv.FormatInt(sz, 10))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, cachedr); err != nil {
		log.Debugf("copy from cached failed: %v", err)
	}
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
		if err := out.Discard(); err != nil {
			log.Errorf("failed to discard cache entry: %v", err)
		}
	} else {
		if err := out.Commit(); err != nil {
			log.Errorf("commit failed: %v", err)
		} else {
			log.Infof("successfully downloaded %v", name)
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
