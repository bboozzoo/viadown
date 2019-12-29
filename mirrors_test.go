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

func TestMirrors(t *testing.T) {
	td, err := ioutil.TempDir("", "viadown-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(td)

	mf := path.Join(td, "foo")
	err = ioutil.WriteFile(mf, []byte(`
# foo bar

http://foo.com
http://bar.tv
`),
		0600)
	assert.NoError(t, err)

	// append entries from file
	m, err := LoadMirrors(mf)
	assert.NoError(t, err)
	assert.Len(t, m, 2)
	assert.EqualValues(t,
		[]string{
			"http://foo.com",
			"http://bar.tv",
		}, m)

	// load file that does not exist
	m, err = LoadMirrors(path.Join(td, "bar"))
	assert.Error(t, err)
	// the list remains unchanged
	assert.Nil(t, m)
}

func TestHasMoreMirrors(t *testing.T) {
	m := Mirrors([]string{"foo", "bar"})
	assert.True(t, HasMoreMirrors(0, m))
	assert.False(t, HasMoreMirrors(1, m))
	assert.False(t, HasMoreMirrors(2, m))
	assert.False(t, HasMoreMirrors(5, m))
}
