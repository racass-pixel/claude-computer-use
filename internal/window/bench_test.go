//go:build windows

package window

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestForegroundBenchmark(t *testing.T) {
	if os.Getenv("CU_BENCH") != "1" {
		t.Skip("set CU_BENCH=1 to run")
	}
	w := New()
	const N = 100

	// Benchmark the new single-HWND Foreground.
	t0 := time.Now()
	for i := 0; i < N; i++ {
		w.Foreground()
	}
	newMs := time.Since(t0).Milliseconds()

	// Benchmark the old List-based path.
	t1 := time.Now()
	for i := 0; i < N; i++ {
		list, _ := w.List()
		for _, wi := range list {
			if wi.Foreground {
				break
			}
		}
	}
	oldMs := time.Since(t1).Milliseconds()

	fmt.Printf("I9 benchmark (%d calls): Foreground (new) = %d ms, List+scan (old) = %d ms\n", N, newMs, oldMs)
}
