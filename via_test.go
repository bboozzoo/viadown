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
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildURL(t *testing.T) {
	assert.Equal(t, "foo/bar", buildURL("foo", "bar"))
	assert.Equal(t, "foo/bar", buildURL("foo/", "bar"))
	assert.Equal(t, "foo/bar", buildURL("foo/", "/bar"))
	assert.Equal(t, "foo/bar", buildURL("foo/", "bar"))
}

func TestCopyHeaders(t *testing.T) {
	h := http.Header{}

	copyHeaders(h, http.Header{}, []string{})
	assert.Len(t, h, 0)
	copyHeaders(h, http.Header{},
		[]string{"Content-Type"})
	assert.Len(t, h, 0)
	copyHeaders(h,
		http.Header{
			"Content-Type": []string{"application/foo"},
		},
		[]string{"Content-Type"})
	assert.Len(t, h, 1)
	assert.Equal(t, "application/foo", h.Get("Content-Type"))

	// h has content type, it should get overwritten now
	copyHeaders(h,
		http.Header{
			"Content-Type": []string{"application/bar"},
		},
		[]string{"Content-Type"})
	assert.Len(t, h, 1)
	assert.Equal(t, "application/bar", h.Get("Content-Type"))

	// add extra headers, 2 in total
	h.Set("Content-Length", "2")

	// try copy, but the header that's in the source set is not listed as
	// one to be copied
	copyHeaders(h,
		http.Header{
			"Content-Encoding": []string{"gzip"},
		},
		[]string{"Content-Type"})
	assert.Len(t, h, 2)
	assert.Equal(t, "application/bar", h.Get("Content-Type"))
	assert.Equal(t, "2", h.Get("Content-Length"))

	// try copy, again
	copyHeaders(h,
		http.Header{
			"Content-Encoding": []string{"gzip"},
		},
		[]string{"Content-Type", "Content-Encoding"})
	assert.Len(t, h, 3)
	assert.Equal(t, "application/bar", h.Get("Content-Type"))
	assert.Equal(t, "2", h.Get("Content-Length"))
	assert.Equal(t, "gzip", h.Get("Content-Encoding"))

}

func TestViaFromCache(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-via-from-cache-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{
		Dir: td,
	}

	rec := httptest.NewRecorder()
	found, err := doFromCache("foo", rec, &c)
	assert.NoError(t, err)
	assert.False(t, found)

	// create a cache object
	cto, err := c.Put("foo")
	cto.Write([]byte("foo"))
	cto.Commit()

	// make it non readable
	cpath := c.getCachePath("foo")
	err = os.Chmod(cpath, 0200)
	assert.NoError(t, err)

	rec = httptest.NewRecorder()
	found, err = doFromCache("foo", rec, &c)
	assert.Error(t, err)
	assert.False(t, found)

	// restore 0600
	err = os.Chmod(cpath, 0600)
	assert.NoError(t, err)

	rec = httptest.NewRecorder()
	found, err = doFromCache("foo", rec, &c)
	assert.NoError(t, err)
	assert.True(t, found)

	assert.Equal(t, "foo", rec.Body.String())
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "3",
		rec.HeaderMap.Get("Content-Length"))
	assert.Equal(t, "application/octet-stream",
		rec.HeaderMap.Get("Content-Type"))

}