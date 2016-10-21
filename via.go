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
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
)

var (
	ErrMirrorFailed = errors.New("mirror failed")
	ErrInternal     = errors.New("internal error")
)

type ViaDownloadServer struct {
	Mirrors *Mirrors
	Cache   *Cache
}

func (v *ViaDownloadServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	upath := r.URL.Path
	log.Debugf("URL path: %v", upath)
	log.Debugf("URL : %v", r.URL)

	cachedr, sz, err := v.Cache.Get(upath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Errorf("cache get failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		log.Debugf("getting from cache, size: %v", sz)
		defer cachedr.Close()

		w.Header().Set("Content-Length", fmt.Sprintf("%v", sz))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, cachedr); err != nil {
			log.Debugf("copy from cached failed: %v", err)
		}
		return
	}

	for _, mirror := range v.Mirrors.List {
		if err := doMirror(mirror, upath, v.Cache, w); err != nil {
			if err == ErrMirrorFailed {
				continue
			} else {
				w.WriteHeader(500)
				break
			}
		} else {
			log.Debugf("download complete")
			break
		}
	}
}

func doMirror(mirror string, upath string, c *Cache, w http.ResponseWriter) error {
	url := buildURL(mirror, upath)
	log.Debugf("target url: %v", url)

	client := http.Client{}
	rsp, err := client.Get(url)
	if err != nil {
		log.Errorf("request to mirror %v failed: %v", mirror, err)
		return ErrMirrorFailed
	}
	log.Debugf("got response: %v", rsp)
	defer rsp.Body.Close()

	if rsp.StatusCode != 200 {
		return ErrMirrorFailed
	} else {
		out, err := c.Put(upath)
		if err != nil {
			return ErrInternal
		}

		tr := io.TeeReader(rsp.Body, out)

		for _, hdr := range []string{"Content-Type", "Content-Length"} {
			hv := rsp.Header.Get(hdr)
			if hv != "" {
				w.Header().Set(hdr, hv)
			}
		}
		w.WriteHeader(http.StatusOK)
		if _, err := io.Copy(w, tr); err != nil {
			log.Errorf("copy failed: %v, discarding cache entry", err)
			if err := out.Discard(); err != nil {
				log.Errorf("failed to discard cache entry: %v", err)
			}
		} else {
			if err := out.Commit(); err != nil {
				log.Errorf("commit failed: %v", err)
			}
		}
	}
	return nil
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
