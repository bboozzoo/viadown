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
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-cache-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{
		Dir: td,
	}

	// non existing entry
	_, _, err = c.Get("foo")
	assert.True(t, os.IsNotExist(err))

	err = ioutil.WriteFile(path.Join(td, "foo"), []byte("foo"),
		0200)
	assert.NoError(t, err)

	_, _, err = c.Get("foo")
	assert.Error(t, err)
	assert.False(t, os.IsNotExist(err))

	// prepare some data
	err = ioutil.WriteFile(path.Join(td, "bar"), []byte("bar"),
		0600)
	assert.NoError(t, err)

	in, sz, err := c.Get("bar")
	assert.NoError(t, err)
	assert.Equal(t, int64(len([]byte("bar"))), sz)
	// read contents
	data, err := ioutil.ReadAll(in)
	assert.NoError(t, err)
	assert.Equal(t, []byte("bar"), data)

	// now try writing
	ct, err := c.Put("zed/foo")
	assert.NoError(t, err)

	_, err = ct.Write([]byte("foo"))
	assert.NoError(t, err)

	err = ct.Commit()
	assert.NoError(t, err)

	data, err = ioutil.ReadFile(path.Join(td, "zed", "foo"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("foo"), data)

	// try to write again, but discard this time
	ct, err = c.Put("zed/foo")
	assert.NoError(t, err)

	_, err = ct.Write([]byte("bar"))
	assert.NoError(t, err)

	err = ct.Abort()
	assert.NoError(t, err)
	// old content should remain
	data, err = ioutil.ReadFile(path.Join(td, "zed", "foo"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("foo"), data)

	// once again, but commit
	ct, err = c.Put("zed/foo")
	assert.NoError(t, err)

	_, err = ct.Write([]byte("bar"))
	assert.NoError(t, err)

	err = ct.Commit()
	assert.NoError(t, err)
	// old content should remain
	data, err = ioutil.ReadFile(path.Join(td, "zed", "foo"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("bar"), data)

	// verify with get
	in, sz, err = c.Get("zed/foo")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), sz)
	data, err = ioutil.ReadAll(in)
	assert.NoError(t, err)
	assert.Equal(t, []byte("bar"), data)
}

func TestCacheCommit(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-cache-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{Dir: td}

	put := func() {
		w, err := c.Put("foo")
		assert.NoError(t, err)
		assert.NotNil(t, w)
		defer w.Commit()

		w.WriteString("hello\n")
	}
	put()

	r, err := ioutil.ReadFile(filepath.Join(td, "foo"))
	assert.NoError(t, err)
	assert.Equal(t, r, []byte("hello\n"))
}

func TestCacheAbort(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-cache-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{Dir: td}

	put := func() {
		w, err := c.Put("foo")
		assert.NoError(t, err)
		assert.NotNil(t, w)
		defer w.Commit()

		w.Abort()
	}
	put()

	_, err = os.Stat(filepath.Join(td, "foo"))
	assert.True(t, os.IsNotExist(err))
}

func TestCacheStats(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-cache-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{Dir: td}
	assert.Equal(t, c.Stats(), CacheStats{})

	err = ioutil.WriteFile(filepath.Join(td, "foo"), []byte("foo"), 0644)
	assert.NoError(t, err)

	rd, _, err := c.Get("foo")
	assert.NoError(t, err)
	assert.NotNil(t, rd)
	defer rd.Close()
	assert.Equal(t, c.Stats(), CacheStats{Hit: 1})

	rd, _, err = c.Get("foo")
	assert.NoError(t, err)
	assert.NotNil(t, rd)
	defer rd.Close()
	assert.Equal(t, c.Stats(), CacheStats{Hit: 2})

	_, _, err = c.Get("bar")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
	assert.Equal(t, c.Stats(), CacheStats{Hit: 2, Miss: 1})

	// calling repeatedly does not change the stats
	assert.Equal(t, c.Stats(), CacheStats{Hit: 2, Miss: 1})
}

func notExist(t *testing.T, p string) {
	_, err := os.Stat(p)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestCachePurgeAll(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-cache-purge-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{Dir: td}

	err = ioutil.WriteFile(filepath.Join(td, "foo"), []byte("foo"), 0644)
	assert.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(td, "bar"), []byte("bar"), 0644)
	assert.NoError(t, err)

	removed, err := c.Purge(PurgeSelector{})
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), removed)

	notExist(t, filepath.Join(td, "foo"))
	notExist(t, filepath.Join(td, "bar"))

	// no files now, still no errors
	removed, err = c.Purge(PurgeSelector{})
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), removed)

	// but the cache dir does
	fi, err := os.Stat(td)
	assert.NoError(t, err)
	assert.True(t, fi.IsDir())
}

func TestCachePurgeSelected(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-cache-purge-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{Dir: td}
	assert.Equal(t, c.Stats(), CacheStats{})

	err = ioutil.WriteFile(filepath.Join(td, "recent-enough"), []byte("foo"), 0644)
	assert.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(td, "too-old"), []byte("bar"), 0644)
	assert.NoError(t, err)

	now := time.Now()
	before := now.Add(-5 * 24 * time.Hour)

	// make too-old old enough
	err = os.Chtimes(filepath.Join(td, "too-old"), now, before.Add(-time.Hour))
	assert.NoError(t, err)

	removed, err := c.Purge(PurgeSelector{OlderThan: before})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), removed)

	notExist(t, filepath.Join(td, "too-old"))
	assert.FileExists(t, filepath.Join(td, "recent-enough"))

	// calling again does not break
	removed, err = c.Purge(PurgeSelector{OlderThan: before})
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), removed)
}

func TestCacheCount(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-cache-count-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{Dir: td}

	err = ioutil.WriteFile(filepath.Join(td, "foo"), []byte("foo"), 0644)
	assert.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(td, "bar"), []byte("bar"), 0644)
	assert.NoError(t, err)

	count, err := c.Count()
	assert.NoError(t, err)
	assert.Equal(t, CacheCount{Items: 2, TotalSize: 6}, count)

	err = os.Remove(filepath.Join(td, "bar"))
	assert.NoError(t, err)

	count, err = c.Count()
	assert.NoError(t, err)
	assert.Equal(t, CacheCount{Items: 1, TotalSize: 3}, count)
}
