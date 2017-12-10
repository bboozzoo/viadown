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
	"bufio"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Mirrors struct {
	List []string
}

func (m *Mirrors) LoadFile(path string) error {
	log.Debugf("loading mirror list from file %v", path)

	f, err := os.Open(path)
	if err != nil {
		log.Errorf("failed to open mirrors file: %v", err)
		return err
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	cnt := 0
	for scan.Scan() {
		if err := scan.Err(); err != nil {
			log.Errorf("failed to read line from mirrors file: %v", err)
			return err
		}

		line := scan.Text()

		if strings.HasPrefix(line, "#") {
			continue
		}
		mirror := strings.TrimSpace(line)

		if len(mirror) == 0 {
			continue
		}
		m.List = append(m.List, mirror)
		cnt++
	}

	log.Infof("got %v mirrors", cnt)
	return nil
}
