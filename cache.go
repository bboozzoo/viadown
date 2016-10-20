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
	"log"
	"os"
	"path"
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
	log.Printf("cache path: %v\n", cpath)
	return cpath
}

func (c *Cache) Get(name string) (io.ReadCloser, int64, error) {
	f, err := os.Open(c.getCachePath(name))
	if err != nil {
		log.Printf("cache get error: %v\n", err)
		if os.IsNotExist(err) {
			c.Stats.Miss += 1
		}
		return nil, 0, err
	}

	c.Stats.Hit += 1

	fi, err := f.Stat()
	if err != nil {
		log.Printf("file stat failed: %v\n", err)
		f.Close()
		return nil, 0, err
	}

	return f, fi.Size(), nil
}

func (c *Cache) Put(name string) (io.WriteCloser, error) {
	// TODO check if path already exists
	cpath := c.getCachePath(name)

	if err := os.MkdirAll(path.Dir(cpath), 0700); err != nil {
		return nil, err
	}

	f, err := os.Create(cpath)
	if err != nil {
		log.Printf("cache put error: %v\n", err)
		return nil, err
	}
	return f, nil
}
