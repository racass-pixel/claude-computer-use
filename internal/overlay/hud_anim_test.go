package overlay

import (
	"math"
	"testing"
)

func TestEaseOutCubicEndpoints(t *testing.T) {
	if v := easeOutCubic(0); v != 0 {
		t.Fatalf("easeOutCubic(0) = %f, want 0", v)
	}
	if v := easeOutCubic(1); v != 1 {
		t.Fatalf("easeOutCubic(1) = %f, want 1", v)
	}
}

func TestEaseOutCubicMonotonic(t *testing.T) {
	prev := 0.0
	for i := 1; i <= 100; i++ {
		v := easeOutCubic(float64(i) / 100)
		if v < prev {
			t.Fatalf("not monotonic: easeOutCubic(%f) = %f < %f", float64(i)/100, v, prev)
		}
		prev = v
	}
}

func TestEaseOutCubicClamp(t *testing.T) {
	if v := easeOutCubic(-0.5); v != 0 {
		t.Fatalf("negative clamped: %f", v)
	}
	if v := easeOutCubic(2.0); v != 1 {
		t.Fatalf("over clamped: %f", v)
	}
}

func TestHudAnimStateSlideIn(t *testing.T) {
	h := 60.0
	// At t=0: fully above
	yOff, alpha := hudAnimState(hudAnimSlideIn, 0, h)
	if yOff != -h {
		t.Fatalf("slide-in t=0: yOffset=%f want %f", yOff, -h)
	}
	if alpha != 0 {
		t.Fatalf("slide-in t=0: alpha=%f want 0", alpha)
	}
	// At t=slideInMs: fully settled
	yOff, alpha = hudAnimState(hudAnimSlideIn, slideInMs, h)
	if yOff != 0 {
		t.Fatalf("slide-in t=end: yOffset=%f want 0", yOff)
	}
	if alpha != 1 {
		t.Fatalf("slide-in t=end: alpha=%f want 1", alpha)
	}
	// At t=slideInMs/2: partially visible
	yOff, alpha = hudAnimState(hudAnimSlideIn, slideInMs/2, h)
	if yOff >= 0 || yOff <= -h {
		t.Fatalf("slide-in t=mid: yOffset=%f should be between -%f and 0", yOff, h)
	}
	if alpha <= 0 || alpha >= 1 {
		t.Fatalf("slide-in t=mid: alpha=%f should be between 0 and 1", alpha)
	}
}

func TestHudAnimStateFadeOut(t *testing.T) {
	// At t=0: fully visible
	_, alpha := hudAnimState(hudAnimFadeOut, 0, 60)
	if alpha != 1 {
		t.Fatalf("fade-out t=0: alpha=%f want 1", alpha)
	}
	// At t=fadeOutMs: gone
	_, alpha = hudAnimState(hudAnimFadeOut, fadeOutMs, 60)
	if alpha != 0 {
		t.Fatalf("fade-out t=end: alpha=%f want 0", alpha)
	}
	// Midway
	_, alpha = hudAnimState(hudAnimFadeOut, fadeOutMs/2, 60)
	if alpha <= 0 || alpha >= 1 {
		t.Fatalf("fade-out t=mid: alpha=%f", alpha)
	}
}

func TestHudAnimStateNone(t *testing.T) {
	yOff, alpha := hudAnimState(hudAnimNone, 100, 60)
	if yOff != 0 || alpha != 1 {
		t.Fatalf("none: yOffset=%f alpha=%f", yOff, alpha)
	}
}

func TestCrossFadeAlphas(t *testing.T) {
	oldA, newA := crossFadeAlphas(0)
	if oldA != 1 || newA != 0 {
		t.Fatalf("crossfade t=0: old=%f new=%f", oldA, newA)
	}
	oldA, newA = crossFadeAlphas(crossFadeMs)
	if oldA != 0 || newA != 1 {
		t.Fatalf("crossfade t=end: old=%f new=%f", oldA, newA)
	}
	// Sum should be ~1 at midpoint
	oldA, newA = crossFadeAlphas(crossFadeMs / 2)
	sum := oldA + newA
	if math.Abs(sum-1) > 0.01 {
		t.Fatalf("crossfade mid sum=%f want ~1", sum)
	}
}

func TestSparkBreathScale(t *testing.T) {
	// At phase where sin=0: scale should be 0.92 + 0.08*1 = 1.0
	s := sparkBreathScale(0) // sin(0) = 0
	if s < 0.99 || s > 1.01 {
		t.Fatalf("sparkBreathScale(0) = %f want ~1.0", s)
	}
	// At phase where sin=-1: scale should be 0.92
	s = sparkBreathScale(-math.Pi / 2) // sin(-pi/2) = -1
	if math.Abs(s-0.92) > 0.01 {
		t.Fatalf("sparkBreathScale(-pi/2) = %f want ~0.92", s)
	}
	// At phase where sin=1: scale should be 1.08
	s = sparkBreathScale(math.Pi / 2) // sin(pi/2) = 1
	if math.Abs(s-1.08) > 0.01 {
		t.Fatalf("sparkBreathScale(pi/2) = %f want ~1.08", s)
	}
}
