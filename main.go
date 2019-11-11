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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	log_syslog "github.com/sirupsen/logrus/hooks/syslog"
	"log/syslog"
)

var (
	optDebug         = flag.Bool("debug", false, "Enable debug logging")
	optCacheRoot     = flag.String("cache-root", "./tmp", "Cache directory path")
	optListenAddr    = flag.String("listen", ":8080", "Listen address")
	optMirrors       = flag.String("mirrors", "", "Mirror list file")
	optTimeout       = flag.Duration("client-timeout", 15*time.Second, "Forward request timeout")
	optVersion       = flag.Bool("version", false, "Show version")
	optSyslog        = flag.Bool("syslog", false, "Enable logging to syslog")
	optPidfile       = flag.String("pidfile", "", "Write self PID to this file")
	optPurgeInterval = flag.Duration("purge-interval", defaultCachePurgeInterval, "Cache purge interval")

	Version = "(unknown)"

	// try purging every 24h
	defaultCachePurgeInterval = 24 * time.Hour
	defaultPurgePolicy        = PurgeSelector{
		// older than 30 days
		OlderThan: 30 * 24 * time.Hour,
	}
)

func main() {

	flag.Parse()

	if *optVersion {
		fmt.Println(Version)
		return
	}

	if *optSyslog {
		h, err := log_syslog.NewSyslogHook("", "", syslog.LOG_INFO, "viadown")
		if err != nil {
			log.Errorf("failed to connect to syslog: %v", err)
		} else {
			log.AddHook(h)
		}
	}

	if *optDebug {
		log.SetLevel(log.DebugLevel)
		log.Debugf("debug logging enabled")
	}

	if *optMirrors == "" {
		log.Errorf("no mirrors, cannot continue")
		os.Exit(1)
	}

	pid := os.Getpid()
	log.Infof("viadown version %v starting... PID: %v", Version, pid)

	if *optPidfile != "" {
		err := ioutil.WriteFile(*optPidfile, []byte(strconv.Itoa(pid)),
			0600)
		if err != nil {
			log.Fatalf("failed to write pid to %s: %v",
				*optPidfile, err)
		}
	}

	m := Mirrors{}
	if err := m.LoadFile(*optMirrors); err != nil {
		log.Errorf("failed to load mirrors from %v: %v",
			*optMirrors, err)
		os.Exit(1)
	}

	log.Infof("cache root: %v", *optCacheRoot)
	cache := Cache{
		Dir: *optCacheRoot,
	}

	cleaner := NewAutomaticCacheCleaner(&cache, *optPurgeInterval, defaultPurgePolicy)

	addr := *optListenAddr
	server := http.Server{
		Addr:    addr,
		Handler: NewViaDownloadServer(&m, &cache, *optTimeout),
	}
	log.Infof("listen on %v", addr)

	listenerrchan := make(chan error)
	sigchan := make(chan os.Signal, 3)

	// wait for SIGINT, SIGTERM, SIGQUIT
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		listenerrchan <- server.ListenAndServe()
	}()

	// start automatic cleaner
	cleaner.Go()
	log.Infof("automatic cache purge every %v, starting now", *optPurgeInterval)

	select {
	case fail := <-listenerrchan:
		log.Fatalf("listen failed: %v", fail)
	case sig := <-sigchan:
		log.Infof("exiting on signal... %s", sig)
	}

	cleaner.Kill()
}
