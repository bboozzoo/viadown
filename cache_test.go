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
	"testing"

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

	err = ct.Discard()
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
