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
	"flag"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
)

var (
	optDebug      = flag.Bool("debug", false, "Enable debug logging")
	optCacheRoot  = flag.String("cache-root", "./tmp", "Cache directory path")
	optListenAddr = flag.String("listen", ":8080", "Listen address")
	optMirrors    = flag.String("mirrors", "", "Mirror list file")
)

func main() {

	flag.Parse()

	if *optDebug {
		log.SetLevel(log.DebugLevel)
		log.Debugf("debug logging enabled")
	}

	if *optMirrors == "" {
		log.Errorf("no mirrors, cannot continue")
		os.Exit(1)
	}

	m := Mirrors{}
	m.LoadFile(*optMirrors)

	log.Infof("cache root: %v", *optCacheRoot)
	cache := Cache{
		Dir: *optCacheRoot,
	}

	addr := *optListenAddr
	server := http.Server{
		Addr: addr,
		Handler: &ViaDownloadServer{
			Mirrors: &m,
			Cache:   &cache,
		},
	}
	log.Infof("listen on %v", addr)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("listen failed: %v", err)
	}
}
