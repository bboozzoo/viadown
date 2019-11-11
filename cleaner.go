// The MIT License (MIT)
//
// Copyright (c) 2019 Maciek Borzecki <maciek.borzecki@gmail.com>
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
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/tomb.v2"
)

type Purger interface {
	Purge(what PurgeSelector) (uint64, error)
}

type AutomaticCacheCleaner struct {
	cache    Purger
	policy   PurgeSelector
	interval time.Duration
	tmb      tomb.Tomb
}

func NewAutomaticCacheCleaner(cache Purger, interval time.Duration, policy PurgeSelector) *AutomaticCacheCleaner {
	return &AutomaticCacheCleaner{
		cache:    cache,
		policy:   policy,
		interval: interval,
	}
}

func (a *AutomaticCacheCleaner) Go() {
	a.tmb.Go(a.periodicPurge)
}

func (a *AutomaticCacheCleaner) periodicPurge() error {
	intervalTimer := time.NewTimer(a.interval)
	defer intervalTimer.Stop()

infiniteLoop:
	for {
		intervalTimer.Reset(a.interval)

		select {
		case <-a.tmb.Dying():
			break infiniteLoop
		case <-intervalTimer.C:
			removed, err := a.cache.Purge(a.policy)
			if err != nil {
				log.Errorf("periodic cache purge failed: %v", err)
			} else {
				log.Infof("periodic cache purge removed %v elements", removed)
			}
		}
	}
	return nil
}

func (a *AutomaticCacheCleaner) Kill() error {
	a.tmb.Kill(nil)
	return a.tmb.Wait()
}
