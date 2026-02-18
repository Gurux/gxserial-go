package gxserial

// --------------------------------------------------------------------------
//
//	Gurux Ltd
//
// Filename:        $HeadURL$
//
// Version:         $Revision$,
//
//	$Date$
//	$Author$
//
// # Copyright (c) Gurux Ltd
//
// ---------------------------------------------------------------------------
//
//	DESCRIPTION
//
// This file is a part of Gurux Device Framework.
//
// Gurux Device Framework is Open Source software; you can redistribute it
// and/or modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation; version 2 of the License.
// Gurux Device Framework is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU General Public License for more details.
//
// More information of Gurux products: https://www.gurux.org
//
// This code is licensed under the GNU General Public License v2.
// Full text may be retrieved at http://www.gnu.org/licenses/gpl-2.0.txt
// ---------------------------------------------------------------------------

import (
	"bytes"
	"sync"
	"time"
)

type synchronousMediaBase struct {
	mu   sync.Mutex
	buf  []byte
	wait chan struct{}
}

func newGXSynchronousMediaBase() *synchronousMediaBase {
	return &synchronousMediaBase{wait: make(chan struct{})}
}

func (b *synchronousMediaBase) Append(p []byte) {
	if len(p) == 0 {
		return
	}
	b.mu.Lock()
	b.buf = append(b.buf, p...)
	old := b.wait
	b.wait = make(chan struct{})
	b.mu.Unlock()
	close(old)
}

func (b *synchronousMediaBase) Get(count int) []byte {
	var ret []byte
	b.mu.Lock()
	if count == -1 || count == len(b.buf) {
		//Copy all data.
		ret = b.buf[:]
		//Clear buffer
		b.buf = b.buf[:0]
	} else {
		ret = b.buf[:count]
		//Copy elements to new slice and remove them from buffer.
		b.buf = b.buf[count:]
	}
	b.mu.Unlock()
	return ret
}

func (b *synchronousMediaBase) Search(pattern []byte, minLen int, maxWait time.Duration) int {
	if minLen < 0 {
		minLen = 0
	}

	deadline := time.Time{}
	switch {
	case maxWait > 0:
		deadline = time.Now().Add(maxWait)
	default:
		// No wait
	}

	if len(pattern) == 0 {
		for {
			b.mu.Lock()
			if len(b.buf) >= minLen {
				b.mu.Unlock()
				return 0
			}
			ch := b.wait
			b.mu.Unlock()

			if maxWait <= 0 {
				return -1
			}
			if !deadline.IsZero() {
				rem := time.Until(deadline)
				if rem <= 0 {
					return -1
				}
				timer := time.NewTimer(rem)
				select {
				case <-ch:
					if !timer.Stop() {
						<-timer.C
					}
					continue
				case <-timer.C:
					return -1
				}
			}
		}
	}

	lastStart := 0
	overlap := len(pattern) - 1
	if overlap < 0 {
		overlap = 0
	}

	for {
		b.mu.Lock()
		start := lastStart
		if start < 0 {
			start = 0
		}
		if start > len(b.buf) {
			start = len(b.buf)
		}
		if len(b.buf) < minLen {
			ch := b.wait
			b.mu.Unlock()

			if maxWait <= 0 {
				return -1
			}
			if !deadline.IsZero() {
				rem := time.Until(deadline)
				if rem <= 0 {
					return -1
				}
				timer := time.NewTimer(rem)
				select {
				case <-ch:
					if !timer.Stop() {
						<-timer.C
					}
					continue
				case <-timer.C:
					return -1
				}
			}
		}

		// Find pattern from buffer.
		if i := bytes.Index(b.buf[start:], pattern); i >= 0 {
			pos := start + i
			b.mu.Unlock()
			return pos + len(pattern)
		}
		// Pattern not found.
		// Keep last bytes that may be part of pattern.
		// For example, if pattern is "abcd" and buffer ends with "ab",
		// we need to keep "ab" in case next buffer starts with "cd".
		// We need to keep len(pattern)-1 bytes for this.
		nextStart := len(b.buf) - overlap
		if nextStart < 0 {
			nextStart = 0
		}
		lastStart = nextStart
		ch := b.wait
		b.mu.Unlock()

		if maxWait <= 0 {
			return -1
		}
		if !deadline.IsZero() {
			rem := time.Until(deadline)
			if rem <= 0 {
				return -1
			}
			timer := time.NewTimer(rem)
			select {
			case <-ch:
				if !timer.Stop() {
					<-timer.C
				}
				continue
			case <-timer.C:
				return -1
			}
		}
	}
}
