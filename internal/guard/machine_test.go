package guard

import (
	"context"
	"testing"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
)

func newMachine(autoPause bool) (*Machine, *[]Transition) {
	h, _ := ParseHotkey("esc esc")
	var log []Transition
	m := New(Config{
		AutoPause: autoPause, MouseThresholdPx: 12, MouseWindow: 300 * time.Millisecond,
		IdleRelease: 2 * time.Second, Hotkey: h, TapWindow: 400 * time.Millisecond,
	}, func(tr Transition) { log = append(log, tr) })
	return m, &log
}

func TestAcquireTouchIdleRelease(t *testing.T) {
	m, log := newMachine(true)
	m.Acquire(at(0))
	if m.State() != Controlling {
		t.Fatalf("state = %v", m.State())
	}
	m.Touch(at(1000))
	m.CheckIdle(at(2500))
	if m.State() != Controlling {
		t.Fatal("must stay controlling within IdleRelease of the last action")
	}
	m.CheckIdle(at(3100))
	if m.State() != Idle || (*log)[len(*log)-1].Reason != ReasonIdle {
		t.Fatalf("idle release failed: %v %v", m.State(), *log)
	}
}

func TestPhysicalKeyPausesInjectedDoesNot(t *testing.T) {
	m, log := newMachine(true)
	m.Acquire(at(0))
	m.HandleEvent(Event{Kind: KeyDown, VK: 0x41, Injected: true, At: at(10)})
	if m.State() != Controlling {
		t.Fatal("injected input must be ignored")
	}
	m.HandleEvent(Event{Kind: KeyDown, VK: 0x41, At: at(20)})
	if m.State() != Paused || (*log)[len(*log)-1].Reason != ReasonPhysicalKey {
		t.Fatalf("physical key must pause: %v", *log)
	}
}

func TestSingleEscIsReservedForTheGesture(t *testing.T) {
	m, _ := newMachine(true)
	m.Acquire(at(0))
	m.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(10)})
	if m.State() != Controlling {
		t.Fatal("a single Esc must not pause (it is part of the hotkey)")
	}
	m.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(200)})
	if m.State() != Paused {
		t.Fatal("Esc Esc must pause")
	}
	m.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(1000)})
	m.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(1200)})
	if m.State() != Controlling {
		t.Fatal("Esc Esc while paused must hand control back")
	}
}

func TestMouseThresholdAndInjectedBaseline(t *testing.T) {
	m, _ := newMachine(true)
	m.Acquire(at(0))
	m.HandleEvent(Event{Kind: MouseMove, Pos: geom.Point{X: 1000, Y: 1000}, Injected: true, At: at(10)})
	m.HandleEvent(Event{Kind: MouseMove, Pos: geom.Point{X: 1004, Y: 1000}, At: at(20)})
	m.HandleEvent(Event{Kind: MouseMove, Pos: geom.Point{X: 1007, Y: 1002}, At: at(40)})
	if m.State() != Controlling {
		t.Fatal("jitter under the threshold must not pause")
	}
	m.HandleEvent(Event{Kind: MouseMove, Pos: geom.Point{X: 1030, Y: 1002}, At: at(60)})
	if m.State() != Paused {
		t.Fatal("moving past the threshold within the window must pause")
	}
}

func TestMouseButtonPausesAndAutoPauseOff(t *testing.T) {
	m, _ := newMachine(true)
	m.Acquire(at(0))
	m.HandleEvent(Event{Kind: MouseDown, At: at(10)})
	if m.State() != Paused {
		t.Fatal("physical click must pause")
	}
	m2, _ := newMachine(false)
	m2.Acquire(at(0))
	m2.HandleEvent(Event{Kind: KeyDown, VK: 0x41, At: at(10)})
	m2.HandleEvent(Event{Kind: MouseDown, At: at(20)})
	if m2.State() != Controlling {
		t.Fatal("auto_pause=false must ignore physical input")
	}
	m2.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(30)})
	m2.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(100)})
	if m2.State() != Paused {
		t.Fatal("the hotkey must work even with auto_pause=false")
	}
}

func TestWaitResumeAndRelease(t *testing.T) {
	m, _ := newMachine(true)
	m.Acquire(at(0))
	m.Pause(at(10), ReasonHotkey)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if m.WaitResume(ctx) {
		t.Fatal("WaitResume must time out while paused")
	}
	go func() { time.Sleep(10 * time.Millisecond); m.Resume(at(20), ReasonPrompt) }()
	if !m.WaitResume(context.Background()) {
		t.Fatal("WaitResume must return true after Resume")
	}
	m.Pause(at(30), ReasonHotkey)
	// Release from Paused: WaitResume should return false (Paused→Idle, not Paused→Controlling).
	go func() { time.Sleep(10 * time.Millisecond); m.Release(at(40)) }()
	if m.WaitResume(context.Background()) {
		t.Fatal("WaitResume must return false on Release (Paused→Idle)")
	}
	if m.State() != Idle || m.IsPaused() {
		t.Fatal("Release must reach Idle from Paused")
	}
	if st := m.Status(); st.State != "idle" || st.Hotkey != "Esc Esc" {
		t.Fatalf("status = %+v", st)
	}
}

func TestUserHoldLatch(t *testing.T) {
	m, _ := newMachine(true)
	m.Acquire(at(0))

	// Hotkey pause sets the latch.
	m.Pause(at(10), ReasonHotkey)
	if !m.UserHold() {
		t.Fatal("hotkey pause must set userHold")
	}
	if st := m.Status(); !st.UserHold {
		t.Fatal("Status must report userHold")
	}

	// Release keeps the latch.
	m.Release(at(20))
	if !m.UserHold() {
		t.Fatal("Release must keep userHold latch")
	}

	// Acquire refuses while latched.
	if m.Acquire(at(30)) {
		t.Fatal("Acquire must refuse while userHold is latched")
	}
	if m.State() != Idle {
		t.Fatal("state should still be idle after refused Acquire")
	}

	// Resume clears the latch.
	m.Resume(at(40), ReasonPrompt)
	if m.UserHold() {
		t.Fatal("Resume must clear userHold latch")
	}

	// Acquire succeeds now.
	if !m.Acquire(at(50)) {
		t.Fatal("Acquire must succeed after Resume clears latch")
	}
	if m.State() != Controlling {
		t.Fatal("state should be controlling after successful Acquire")
	}
}

func TestUserHoldLatchPhysicalInput(t *testing.T) {
	m, _ := newMachine(true)
	m.Acquire(at(0))

	// Physical key pause sets the latch.
	m.HandleEvent(Event{Kind: KeyDown, VK: 0x41, At: at(10)})
	if m.State() != Paused {
		t.Fatal("physical key must pause")
	}
	if !m.UserHold() {
		t.Fatal("physical key pause must set userHold")
	}

	// Esc Esc while paused resumes and clears the latch.
	m.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(100)})
	m.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(200)})
	if m.State() != Controlling {
		t.Fatal("Esc Esc while paused must resume")
	}
	if m.UserHold() {
		t.Fatal("hotkey resume must clear userHold")
	}
}

func TestUserHoldLatchCtlReason(t *testing.T) {
	m, _ := newMachine(false)
	m.Acquire(at(0))

	// Ctl pause sets the latch.
	m.Pause(at(10), ReasonCtl)
	if !m.UserHold() {
		t.Fatal("ctl pause must set userHold")
	}
}
