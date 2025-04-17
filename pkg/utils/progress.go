package utils

import (
	"fmt"
	"sync/atomic"
	"time"
)

type ProgressReporter struct {
	total     int64
	processed int64
	startTime time.Time
	lastPrint time.Time
}

func NewProgressReporter(total int) *ProgressReporter {
	return &ProgressReporter{
		total:     int64(total),
		processed: 0,
		startTime: time.Now(),
		lastPrint: time.Now(),
	}
}

func (p *ProgressReporter) SetTotal(total int) {
	atomic.StoreInt64(&p.total, int64(total))
}

func (p *ProgressReporter) Increment() {
	atomic.AddInt64(&p.processed, 1)
	p.MaybePrint()
}

func (p *ProgressReporter) IncrementBy(n int) {
	atomic.AddInt64(&p.processed, int64(n))
	p.MaybePrint()
}

func (p *ProgressReporter) MaybePrint() {
	now := time.Now()
	
	// Only print max once per second
	if now.Sub(p.lastPrint) < time.Second {
		return
	}
	
	p.lastPrint = now
	p.PrintProgress()
}

func (p *ProgressReporter) PrintProgress() {
	processed := atomic.LoadInt64(&p.processed)
	total := atomic.LoadInt64(&p.total)
	
	if total == 0 {
		fmt.Printf("Processed %d files\n", processed)
		return
	}
	
	percentage := float64(processed) / float64(total) * 100
	elapsed := time.Since(p.startTime)
	
	var eta time.Duration
	if processed > 0 {
		eta = time.Duration(float64(elapsed) / float64(processed) * float64(total-processed))
	}
	
	fmt.Printf("Progress: %d/%d (%.1f%%) - Elapsed: %s, ETA: %s\n",
		processed, total, percentage, formatDuration(elapsed), formatDuration(eta))
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	
	if d < time.Minute {
		return fmt.Sprintf("%ds", d/time.Second)
	} else if d < time.Hour {
		m := d / time.Minute
		d -= m * time.Minute
		return fmt.Sprintf("%dm %ds", m, d/time.Second)
	} else {
		h := d / time.Hour
		d -= h * time.Hour
		m := d / time.Minute
		d -= m * time.Minute
		return fmt.Sprintf("%dh %dm %ds", h, m, d/time.Second)
	}
}