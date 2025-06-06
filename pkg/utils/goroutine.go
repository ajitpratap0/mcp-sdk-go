package utils

import (
	"runtime"
	"testing"
	"time"
)

// GoroutineLeakDetector helps detect goroutine leaks in tests
type GoroutineLeakDetector struct {
	t              *testing.T
	initialCount   int
	allowedGrowth  int
	checkInterval  time.Duration
	stabilizeDelay time.Duration
}

// NewGoroutineLeakDetector creates a new goroutine leak detector
func NewGoroutineLeakDetector(t *testing.T) *GoroutineLeakDetector {
	return &GoroutineLeakDetector{
		t:              t,
		allowedGrowth:  0,
		checkInterval:  100 * time.Millisecond,
		stabilizeDelay: 200 * time.Millisecond,
	}
}

// Start records the initial goroutine count
func (d *GoroutineLeakDetector) Start() {
	// Allow goroutines to stabilize
	time.Sleep(d.stabilizeDelay)
	d.initialCount = runtime.NumGoroutine()
	d.t.Logf("Starting goroutine count: %d", d.initialCount)
}

// Check verifies that goroutine count hasn't grown beyond allowed threshold
func (d *GoroutineLeakDetector) Check() {
	// Allow goroutines to finish
	time.Sleep(d.stabilizeDelay)

	// Check multiple times to ensure stability
	var counts []int
	for i := 0; i < 3; i++ {
		count := runtime.NumGoroutine()
		counts = append(counts, count)
		if i < 2 {
			time.Sleep(d.checkInterval)
		}
	}

	// Use the minimum count (some goroutines might be in cleanup)
	finalCount := counts[0]
	for _, c := range counts {
		if c < finalCount {
			finalCount = c
		}
	}

	leaked := finalCount - d.initialCount
	if leaked > d.allowedGrowth {
		d.t.Errorf("Goroutine leak detected: started with %d, ended with %d (leaked: %d, allowed: %d)",
			d.initialCount, finalCount, leaked, d.allowedGrowth)

		// Print stack traces for debugging
		buf := make([]byte, 1<<20)
		stackLen := runtime.Stack(buf, true)
		d.t.Logf("Current goroutine stack traces:\n%s", buf[:stackLen])
	} else {
		d.t.Logf("No goroutine leak: started with %d, ended with %d", d.initialCount, finalCount)
	}
}

// SetAllowedGrowth sets the number of goroutines allowed to grow
func (d *GoroutineLeakDetector) SetAllowedGrowth(n int) *GoroutineLeakDetector {
	d.allowedGrowth = n
	return d
}

// SetStabilizeDelay sets the delay to allow goroutines to stabilize
func (d *GoroutineLeakDetector) SetStabilizeDelay(delay time.Duration) *GoroutineLeakDetector {
	d.stabilizeDelay = delay
	return d
}
