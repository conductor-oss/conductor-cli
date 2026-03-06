/*
 * Copyright 2026 Conductor Authors.
 * <p>
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 * <p>
 * http://www.apache.org/licenses/LICENSE-2.0
 * <p>
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const (
	barWidth       = 30
	filledChar     = "█"
	emptyChar      = "░"
	renderInterval = 100 * time.Millisecond
)

// ProgressBar tracks and renders download progress to stderr.
type ProgressBar struct {
	total       int64
	current     int64
	desc        string
	mu          sync.Mutex
	lastRender  time.Time
	w           io.Writer
}

// NewProgressBar creates a new progress bar. If total <= 0, it renders without a bar.
func NewProgressBar(total int64, desc string) *ProgressBar {
	return &ProgressBar{
		total: total,
		desc:  desc,
		w:     os.Stderr,
	}
}

// Add advances the progress bar by n bytes.
func (p *ProgressBar) Add(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current += n
	now := time.Now()
	if now.Sub(p.lastRender) >= renderInterval {
		p.render()
		p.lastRender = now
	}
}

// Finish renders the final state with a newline.
func (p *ProgressBar) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.render()
	fmt.Fprintln(p.w)
}

func (p *ProgressBar) render() {
	if p.total > 0 {
		pct := float64(p.current) / float64(p.total)
		if pct > 1 {
			pct = 1
		}
		filled := int(pct * float64(barWidth))
		bar := ""
		for i := 0; i < barWidth; i++ {
			if i < filled {
				bar += filledChar
			} else {
				bar += emptyChar
			}
		}
		fmt.Fprintf(p.w, "\r%s  [%s]  %3.0f%%  %s/%s",
			p.desc, bar, pct*100,
			FormatBytes(p.current), FormatBytes(p.total))
	} else {
		fmt.Fprintf(p.w, "\r%s  %s downloaded", p.desc, FormatBytes(p.current))
	}
}

// Reader wraps an io.Reader and reports progress on each Read.
type Reader struct {
	reader io.Reader
	bar    *ProgressBar
}

// NewReader wraps r with a progress bar. desc is shown as a label.
// If total <= 0, a simple byte counter is shown instead of a bar.
func NewReader(r io.Reader, total int64, desc string) (*Reader, *ProgressBar) {
	bar := NewProgressBar(total, desc)
	return &Reader{reader: r, bar: bar}, bar
}

func (r *Reader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.bar.Add(int64(n))
	}
	return n, err
}

// FormatBytes returns a human-readable byte string (e.g. "12.3 MB").
func FormatBytes(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
