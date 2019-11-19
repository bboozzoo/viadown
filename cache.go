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
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type ReadSeekCloser interface {
	io.ReadSeeker
	io.Closer
}

type PurgeEvent struct {
	When    time.Time
	Removed uint64
}

const PurgeHistoryMaxCount = 5

type CacheStats struct {
	Hit          int
	Miss         int
	PurgeHistory []PurgeEvent
}

type CacheCount struct {
	// Items is the count of all items (files) in the cache
	Items uint64
	// TotalSize is the aggregate size of all items in bytes
	TotalSize uint64
}

type Cache struct {
	Dir       string
	dirLock   sync.Mutex
	stats     CacheStats
	statsLock sync.Mutex
}

func (c *Cache) getCachePath(name string) string {
	cpath := path.Join(c.Dir, name)
	log.Debugf("cache path: %v", cpath)
	return cpath
}

func (c *Cache) Get(name string) (ReadSeekCloser, int64, error) {
	c.dirLock.Lock()
	defer c.dirLock.Unlock()

	f, err := os.Open(c.getCachePath(name))
	if err != nil {
		if os.IsNotExist(err) {
			c.miss()
		}
		log.Errorf("cache get error: %v", err)
		return nil, 0, err
	}

	c.hit()

	fi, err := f.Stat()
	if err != nil {
		log.Errorf("file %s stat failed: %v", f.Name(), err)
		f.Close()
		return nil, 0, err
	}

	return f, fi.Size(), nil
}

func (c *Cache) Put(name string) (*CacheTemporaryObject, error) {
	c.dirLock.Lock()
	defer c.dirLock.Unlock()

	cpath := c.getCachePath(name)

	if err := os.MkdirAll(path.Dir(cpath), 0700); err != nil {
		return nil, err
	}

	f, err := ioutil.TempFile(path.Dir(cpath), path.Base(cpath)+".part.")
	if err != nil {
		log.Errorf("cache put for %v error: %v", cpath, err)
		return nil, err
	}

	ct := CacheTemporaryObject{
		File:       f,
		targetName: cpath,
		curName:    f.Name(),
	}
	return &ct, nil
}

func (c *Cache) Stats() CacheStats {
	c.statsLock.Lock()
	defer c.statsLock.Unlock()
	return c.stats
}

func (c *Cache) hit() {
	c.statsLock.Lock()
	defer c.statsLock.Unlock()
	c.stats.Hit++
}

func (c *Cache) miss() {
	c.statsLock.Lock()
	defer c.statsLock.Unlock()
	c.stats.Miss++
}

func (c *Cache) Count() (CacheCount, error) {
	c.dirLock.Lock()
	defer c.dirLock.Unlock()

	count := CacheCount{}
	walkCount := func(name string, fi os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "cannot process path %v", name)
		}
		if fi.IsDir() {
			return nil
		}
		count.Items++
		count.TotalSize += uint64(fi.Size())
		return nil
	}
	err := filepath.Walk(c.Dir, walkCount)
	return count, err
}

type PurgeSelector struct {
	OlderThan time.Duration
}

func (c *Cache) addPurgeEvent(event PurgeEvent) {
	if event.When.IsZero() {
		return
	}
	history := c.stats.PurgeHistory
	if len(history) >= PurgeHistoryMaxCount {
		history = history[1:]
	}
	c.stats.PurgeHistory = append(history, event)
}

func (c *Cache) Purge(what PurgeSelector) (removed uint64, err error) {
	c.dirLock.Lock()
	defer c.dirLock.Unlock()

	now := time.Now()

	log.Infof("cache purge: older than %v", what.OlderThan)

	var rmError error
	walkPurgeSelected := func(name string, fi os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "cannot process path %v", name)
		}
		if fi.IsDir() {
			return nil
		}
		remove := true
		if what.OlderThan != 0 && now.Sub(fi.ModTime()) < what.OlderThan {
			remove = false
		}
		if remove {
			log.Infof("removing %v", name)
			err := os.Remove(name)
			if err != nil && rmError == nil {
				rmError = errors.Wrapf(err, "cannot remove entry %v", name)
			}
			if err == nil {
				removed++
			}
		}
		return nil
	}
	err = filepath.Walk(c.Dir, walkPurgeSelected)
	if err == nil {
		c.addPurgeEvent(PurgeEvent{When: now, Removed: removed})
	}
	return removed, err
}

type CacheTemporaryObject struct {
	*os.File
	targetName string
	curName    string
	aborted    bool
}

func (ct *CacheTemporaryObject) Commit() error {
	if ct.aborted {
		return nil
	}

	if err := ct.Close(); err != nil {
		return err
	}

	if err := os.Rename(ct.curName, ct.targetName); err != nil {
		log.Errorf("rename %v -> %v failed: %v",
			ct.curName, ct.targetName, err)
		return err
	}
	log.Debugf("commited cache entry %v to %v", ct.curName, ct.targetName)
	return nil
}

func (ct *CacheTemporaryObject) Abort() error {
	log.Debugf("discard entry %v", ct.curName)
	ct.aborted = true

	if err := ct.Close(); err != nil {
		return err
	}

	if err := os.Remove(ct.curName); err != nil {
		return err
	}
	return nil
}
