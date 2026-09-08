package overlay

import "math"

// easeOutCubic returns 1 - (1-t)^3, clamped to [0,1].
func easeOutCubic(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	t = 1 - t
	return 1 - t*t*t
}

// hudAnimKind describes which HUD transition is active.
type hudAnimKind int

const (
	hudAnimNone    hudAnimKind = iota
	hudAnimSlideIn             // show: slide from y=-h to restY, alpha 0->1, 220 ms
	hudAnimFadeOut             // hide: alpha 1->0, 180 ms
)

const (
	slideInMs   = 220
	fadeOutMs   = 180
	crossFadeMs = 160
)

// hudAnimState returns (yOffset from restY, alpha) for a slide-in or fade-out.
// elapsed is in milliseconds. For slideIn, h is the HUD height.
func hudAnimState(kind hudAnimKind, elapsedMs, h float64) (yOffset float64, alpha float64) {
	switch kind {
	case hudAnimSlideIn:
		t := elapsedMs / slideInMs
		e := easeOutCubic(t)
		// y goes from -(h + restY) to 0 (offset relative to rest position)
		yOffset = -(h) * (1 - e)
		alpha = e
		if t >= 1 {
			yOffset = 0
			alpha = 1
		}
		return yOffset, alpha
	case hudAnimFadeOut:
		t := elapsedMs / fadeOutMs
		if t >= 1 {
			return 0, 0
		}
		return 0, 1 - t
	default:
		return 0, 1
	}
}

// crossFadeAlphas returns (oldAlpha, newAlpha) for a crossfade at elapsedMs.
// Both are in [0,1] and sum to ~1 (linear crossfade).
func crossFadeAlphas(elapsedMs float64) (oldA, newA float64) {
	t := elapsedMs / crossFadeMs
	if t >= 1 {
		return 0, 1
	}
	if t <= 0 {
		return 1, 0
	}
	return 1 - t, t
}

// sparkBreathScale returns the spark radius multiplier (0.92..1.08) synced with border breathing.
func sparkBreathScale(borderPhase float64) float64 {
	return 0.92 + 0.08*(math.Sin(borderPhase)+1)
}
