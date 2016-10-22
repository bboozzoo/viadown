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

	log "github.com/Sirupsen/logrus"
)

type Cache struct {
	Dir   string
	Stats struct {
		Hit  int
		Miss int
	}
}

func (c *Cache) getCachePath(name string) string {
	cpath := path.Join(c.Dir, name)
	log.Debugf("cache path: %v", cpath)
	return cpath
}

func (c *Cache) Get(name string) (io.ReadCloser, int64, error) {
	f, err := os.Open(c.getCachePath(name))
	if err != nil {
		if os.IsNotExist(err) {
			log.Infof("cache miss for %v", name)
			c.Stats.Miss++
		}
		log.Errorf("cache get error: %v", err)
		return nil, 0, err
	}

	log.Infof("cache hit for %v", name)
	c.Stats.Hit++

	fi, err := f.Stat()
	if err != nil {
		log.Errorf("file %s stat failed: %v", f.Name(), err)
		f.Close()
		return nil, 0, err
	}

	return f, fi.Size(), nil
}

func (c *Cache) Put(name string) (*CacheTemporaryObject, error) {
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

type CacheTemporaryObject struct {
	*os.File
	targetName string
	curName    string
}

func (ct *CacheTemporaryObject) Commit() error {
	if err := ct.Close(); err != nil {
		return err
	}

	log.Debugf("committing entry %v to %v", ct.curName, ct.targetName)
	if err := os.Rename(ct.curName, ct.targetName); err != nil {
		return err
	}
	return nil
}

func (ct *CacheTemporaryObject) Discard() error {
	log.Debugf("discard entry %v", ct.curName)
	if err := ct.Close(); err != nil {
		return err
	}

	if err := os.Remove(ct.curName); err != nil {
		return err
	}
	return nil
}
