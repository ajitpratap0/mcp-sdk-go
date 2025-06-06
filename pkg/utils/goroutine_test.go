package utils

import (
	"testing"
)

// TestGoroutineLeakDetector tests the leak detector itself
func TestGoroutineLeakDetector(t *testing.T) {
	t.Run("NoLeak", func(t *testing.T) {
		detector := NewGoroutineLeakDetector(t)
		detector.Start()

		// Do some work that doesn't leak
		ch := make(chan struct{})
		go func() {
			ch <- struct{}{}
		}()
		<-ch

		detector.Check()
	})

	t.Run("DetectsLeak", func(t *testing.T) {
		// Create a mock test to capture the error
		mockT := &testing.T{}
		detector := NewGoroutineLeakDetector(mockT)
		detector.Start()

		// Intentionally leak a goroutine
		go func() {
			select {} // Block forever
		}()

		detector.Check()

		if !mockT.Failed() {
			t.Error("Expected leak detector to fail but it didn't")
		}
	})
}
