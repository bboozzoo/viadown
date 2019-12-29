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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestDoFromCache(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-via-from-cache-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{
		Dir: td,
	}

	rec := httptest.NewRecorder()
	r, err := http.NewRequest(http.MethodGet, "/foo", nil)
	assert.NoError(t, err)
	found, err := doFromCache("foo", rec, r, &c)
	assert.NoError(t, err)
	assert.False(t, found)

	// create a cache object
	cto, err := c.Put("foo")
	assert.NoError(t, err)
	cto.Write([]byte("foo"))
	cto.Commit()

	// make it non readable
	cpath := c.getCachePath("foo")
	err = os.Chmod(cpath, 0200)
	assert.NoError(t, err)

	r, err = http.NewRequest(http.MethodGet, "/foo", nil)
	assert.NoError(t, err)
	rec = httptest.NewRecorder()
	found, err = doFromCache("foo", rec, r, &c)
	assert.Error(t, err)
	assert.False(t, found)

	// restore 0600
	err = os.Chmod(cpath, 0600)
	assert.NoError(t, err)

	r, err = http.NewRequest(http.MethodGet, "/foo", nil)
	assert.NoError(t, err)
	rec = httptest.NewRecorder()
	found, err = doFromCache("foo", rec, r, &c)
	assert.NoError(t, err)
	assert.True(t, found)

	assert.Equal(t, "foo", rec.Body.String())
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "3",
		rec.Result().Header.Get("Content-Length"))
	assert.Equal(t, "application/octet-stream",
		rec.Result().Header.Get("Content-Type"))

}

type mockUpstreamResponse struct {
	Code int    // if non 0, HTTP status code to be sent, otherwise responds with 200
	Body string // if len() > 0, data to be sent
}

type MockUpstreamServer struct {
	responses map[string]mockUpstreamResponse
	t         *testing.T
}

func newMockUpstreamServer(t *testing.T, rsps map[string]mockUpstreamResponse) *httptest.Server {
	ms := &MockUpstreamServer{
		t:         t,
		responses: rsps,
	}
	srv := httptest.NewServer(ms)
	require.NotNil(t, srv)
	return srv
}

func (m *MockUpstreamServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u := r.URL.String()
	mr, ok := m.responses[u]
	if !ok {
		assert.FailNowf(m.t, "no mocked response", "requested URL: %q", u)
	}
	if mr.Code != 0 {
		w.WriteHeader(mr.Code)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	if len(mr.Body) != 0 {
		w.Write([]byte(mr.Body))
	}
}

func TestDoFromUpstream4xx(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-via-from-cache-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{
		Dir: td,
	}

	// upstream returns 404
	srv := newMockUpstreamServer(t, map[string]mockUpstreamResponse{
		"/foo": {Code: http.StatusNotFound},
	})
	defer srv.Close()

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/foo", nil)
	err = doFromUpstream("foo", &http.Client{}, req, rec, &c)
	require.NotNil(t, err)
	assert.Regexp(t, `(?m)^bad upstream ".*" status 404, .*$`, err)
	assert.False(t, rec.Flushed)

}

func TestDoFromUpstream3xx(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-via-from-cache-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{
		Dir: td,
	}

	// upstream returns 304 and data
	srv := newMockUpstreamServer(t, map[string]mockUpstreamResponse{
		"/bar": {Code: http.StatusNotModified},
	})
	defer srv.Close()

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/bar", nil)
	err = doFromUpstream("bar", &http.Client{}, req, rec, &c)
	require.NotNil(t, err)
	assert.Regexp(t, `(?m)^bad upstream ".*" status 304, .*$`, err)
	assert.False(t, rec.Flushed)

	// check the error
	var badStatusErr *errUpstreamBadStatus
	require.True(t, errors.As(err, &badStatusErr))
	require.NotNil(t, badStatusErr.Rsp)
	assert.Equal(t, badStatusErr.Rsp.StatusCode, http.StatusNotModified)

	// since upstream code was not 200, there should be no data in cache
	_, _, err = c.Get("bar")
	assert.True(t, os.IsNotExist(err))
}

func TestDoFromUpstreamHappy(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-via-from-cache-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	c := Cache{
		Dir: td,
	}

	// upstream returns 200 and data
	srv := newMockUpstreamServer(t, map[string]mockUpstreamResponse{
		"/foo": {Code: http.StatusOK, Body: "foo"},
	})
	defer srv.Close()

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/foo", nil)
	err = doFromUpstream("foo", &http.Client{}, req, rec, &c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []byte("foo"), rec.Body.Bytes())

	// data should have made it to cache
	in, _, err := c.Get("foo")
	assert.NoError(t, err)
	data, _ := ioutil.ReadAll(in)
	assert.Equal(t, []byte("foo"), data)
}

type viaFixture struct {
	via      *ViaDownloadServer
	cache    *Cache
	td       string
	cacheDir string
	vfsDir   string
}

func (f *viaFixture) Cleanup() {
	os.RemoveAll(f.td)
}

func setupVia(t *testing.T, mirrors Mirrors) viaFixture {
	td, err := ioutil.TempDir("", "viadown-test-")
	require.NoError(t, err)

	cacheDir := filepath.Join(td, "cache")
	err = os.MkdirAll(cacheDir, 0755)
	require.NoError(t, err)

	vfsDir := filepath.Join(td, "vfs")
	err = os.MkdirAll(cacheDir, 0755)
	require.NoError(t, err)

	c := Cache{
		Dir: cacheDir,
	}

	vfs := http.Dir(vfsDir)

	via := NewViaDownloadServer(mirrors, &c, 10*time.Second, vfs)

	return viaFixture{
		via:      via,
		cache:    &c,
		td:       td,
		cacheDir: cacheDir,
		vfsDir:   vfsDir,
	}
}

func makeFile(t *testing.T, path string, data []byte) {
	prefix := filepath.Dir(path)
	err := os.MkdirAll(prefix, 0755)
	require.NoError(t, err)

	err = ioutil.WriteFile(path, data, 0644)
	require.NoError(t, err)
}

func TestViaStatic(t *testing.T) {
	fixture := setupVia(t, nil)
	defer fixture.Cleanup()
	via := fixture.via

	assert.HTTPError(t, via.ServeHTTP, http.MethodGet, "/_viadown/foo", nil)
	assert.HTTPRedirect(t, via.ServeHTTP, http.MethodGet, "/_viadown", nil)
	makeFile(t, filepath.Join(fixture.vfsDir, "ok"), nil)
	assert.HTTPSuccess(t, via.ServeHTTP, http.MethodGet, "/_viadown/ok", nil)
}

func TestViaFromCacheTrivial(t *testing.T) {
	fixture := setupVia(t, nil)
	defer fixture.Cleanup()
	via := fixture.via

	assert.HTTPError(t, via.ServeHTTP, http.MethodGet, "/foo", nil)
	makeFile(t, filepath.Join(fixture.cacheDir, "ok"), []byte("this is cached body"))
	assert.HTTPBodyContains(t, via.ServeHTTP, http.MethodGet, "/ok", nil, "this is cached body")
}

func TestViaFromUpstreamSingle(t *testing.T) {
	// upstream returns 200 and data
	srv := newMockUpstreamServer(t, map[string]mockUpstreamResponse{
		"/ok": {Code: http.StatusOK, Body: "this is upstream"},
	})
	defer srv.Close()

	fixture := setupVia(t, []string{srv.URL})
	cache, via := fixture.cache, fixture.via

	assert.HTTPBodyContains(t, via.ServeHTTP, http.MethodGet, "/ok", nil, "this is upstream")
	in, _, err := cache.Get("ok")
	require.NoError(t, err)
	data, _ := ioutil.ReadAll(in)
	assert.Equal(t, []byte("this is upstream"), data)
}

func TestViaFromUpstreamMany(t *testing.T) {
	srv1 := newMockUpstreamServer(t, map[string]mockUpstreamResponse{
		"/foo": {Code: http.StatusNotFound},
	})
	defer srv1.Close()

	srv2 := newMockUpstreamServer(t, map[string]mockUpstreamResponse{
		"/foo": {Code: http.StatusOK, Body: "this is srv2"},
	})
	defer srv2.Close()

	fixture := setupVia(t, []string{srv1.URL, srv2.URL})
	cache, via := fixture.cache, fixture.via

	assert.HTTPBodyContains(t, via.ServeHTTP, http.MethodGet, "/foo", nil, "this is srv2")
	in, _, err := cache.Get("foo")
	require.NoError(t, err)
	data, _ := ioutil.ReadAll(in)
	assert.Equal(t, []byte("this is srv2"), data)
}

func TestViaFromUpstreamBadMirror(t *testing.T) {
	fixture := setupVia(t, []string{"http://bar-mirror.local:1234"})
	cache, via := fixture.cache, fixture.via

	assert.HTTPError(t, via.ServeHTTP, http.MethodGet, "/foo", nil)
	_, _, err := cache.Get("foo")
	assert.True(t, os.IsNotExist(err))
}

func TestViaFromUpstreamNotModified(t *testing.T) {
	srv := newMockUpstreamServer(t, map[string]mockUpstreamResponse{
		"/foo": {Code: http.StatusNotModified},
	})
	defer srv.Close()

	fixture := setupVia(t, []string{srv.URL})
	via := fixture.via

	makeFile(t, filepath.Join(fixture.cacheDir, "foo"), []byte("this is cached body"))

	rec := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/foo", nil)
	req.Header.Add("If-Modified-Since", "Wed, 21 Oct 2015 07:28:00 GMT")
	require.NoError(t, err)
	via.ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusNotModified)
	assert.Empty(t, rec.Body.String())
}
