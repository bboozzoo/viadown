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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockPurger struct {
	purgeFunc func(what PurgeSelector) (uint64, error)
}

func (m *mockPurger) Purge(what PurgeSelector) (uint64, error) {
	if m.purgeFunc == nil {
		panic("purge callback unset")
	}
	return m.purgeFunc(what)
}

func TestCacheCleanerHappy(t *testing.T) {
	policy := PurgeSelector{OlderThan: 123}

	calls := 0
	m := mockPurger{
		purgeFunc: func(what PurgeSelector) (uint64, error) {
			calls++
			assert.Equal(t, policy, what)
			return 0, nil
		},
	}

	a := NewAutomaticCacheCleaner(&m, 5*time.Millisecond, policy)
	a.Go()
	time.Sleep(20 * time.Millisecond)
	err := a.Kill()
	assert.Nil(t, err)
	assert.True(t, calls > 0)
}

func TestCacheCleanerErrorSwallow(t *testing.T) {
	policy := PurgeSelector{OlderThan: 123}

	calls := 0
	m := mockPurger{
		purgeFunc: func(what PurgeSelector) (uint64, error) {
			calls++
			return 0, fmt.Errorf("failing call %v", calls)
		},
	}

	a := NewAutomaticCacheCleaner(&m, 5*time.Millisecond, policy)
	a.Go()
	time.Sleep(20 * time.Millisecond)
	err := a.Kill()
	assert.Nil(t, err)
	assert.True(t, calls > 0)
}
