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
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func init() {
	ll := asLogrusLogger(log)
	ll.SetOutput(ioutil.Discard)
}

func TestLogNoPanic(t *testing.T) {
	log.Debugf("foo %s", "bar")
}

func mockLoggerOutput(out io.Writer) (cleanup func()) {
	ll := asLogrusLogger(log)
	old := ll.Out
	ll.SetOutput(out)

	return func() { ll.SetOutput(old) }
}

func TestDebugLog(t *testing.T) {
	var buf bytes.Buffer

	restore := mockLoggerOutput(&buf)
	defer restore()

	log.Debugf("foo")
	assert.Empty(t, buf.String())

	EnableDebugLog()

	log.Debugf("foo")
	assert.Contains(t, buf.String(), "foo\n")
}
