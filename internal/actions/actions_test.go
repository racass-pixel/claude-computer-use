package actions

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/platform/fake"
)

// failOnNthKeyDown wraps fake.Input and fails the nth (1-based) KeyDown call.
type failOnNthKeyDown struct {
	*fake.Input
	failAt int
	n      int
}

func (f *failOnNthKeyDown) KeyDown(vk uint16) error {
	f.n++
	if f.n == f.failAt {
		return fmt.Errorf("boom: KeyDown call %d", f.n)
	}
	return f.Input.KeyDown(vk)
}

// failOnNthMouseMove wraps fake.Input and fails the nth (1-based) MouseMove call.
type failOnNthMouseMove struct {
	*fake.Input
	failAt int
	n      int
}

func (f *failOnNthMouseMove) MouseMove(p geom.Point) error {
	f.n++
	if f.n == f.failAt {
		return fmt.Errorf("boom: MouseMove call %d", f.n)
	}
	return f.Input.MouseMove(p)
}

// failGetTextClipboard wraps fake.Clipboard and always fails GetText.
type failGetTextClipboard struct {
	*fake.Clipboard
}

func (c *failGetTextClipboard) GetText() (string, error) {
	return "", fmt.Errorf("boom: GetText")
}

func newActor() (*Actor, *fake.Input, *fake.Clipboard) {
	in := &fake.Input{}
	clip := &fake.Clipboard{Text: "old"}
	return &Actor{In: in, Clip: clip, Sleep: func(time.Duration) {}, PasteThreshold: 200}, in, clip
}

func TestClickMovesThenPressesNTimes(t *testing.T) {
	a, in, _ := newActor()
	if err := a.Click(geom.Point{X: 10, Y: 20}, platform.ButtonLeft, 2, nil); err != nil {
		t.Fatal(err)
	}
	want := "move 10,20|down left|up left|down left|up left"
	if got := strings.Join(in.Calls, "|"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestClickWithModifiersWrapsPress(t *testing.T) {
	a, in, _ := newActor()
	_ = a.Click(geom.Point{X: 1, Y: 1}, platform.ButtonRight, 1, []uint16{input.VK_CONTROL})
	want := "key_down 17|move 1,1|down right|up right|key_up 17"
	if got := strings.Join(in.Calls, "|"); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestDragInterpolatesAndReleasesAtTarget(t *testing.T) {
	a, in, _ := newActor()
	_ = a.Drag(geom.Point{X: 0, Y: 0}, geom.Point{X: 100, Y: 50}, platform.ButtonLeft, 250*time.Millisecond)
	calls := in.Calls
	if calls[0] != "move 0,0" || calls[1] != "down left" || calls[len(calls)-1] != "up left" || calls[len(calls)-2] != "move 100,50" {
		t.Fatalf("bad drag sequence: %v", calls)
	}
	if len(calls) < 2+8+1 {
		t.Fatalf("drag must have at least 8 intermediate moves, got %d calls", len(calls))
	}
}

func TestChordOrder(t *testing.T) {
	a, in, _ := newActor()
	c, _ := input.ParseChord("ctrl+shift+t")
	_ = a.Chord(c, 0)
	want := "key_down 17|key_down 16|key_down 84|key_up 84|key_up 16|key_up 17"
	if got := strings.Join(in.Calls, "|"); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestTypeAutoUsesUnicodeForShortAndPasteForLong(t *testing.T) {
	a, in, clip := newActor()
	_ = a.Type("hello", "auto", 0)
	if strings.Join(in.Calls, "|") != "type hello" {
		t.Fatalf("short text: %v", in.Calls)
	}
	long := strings.Repeat("x", 300)
	in.Calls = nil
	_ = a.Type(long, "auto", 0)
	joined := strings.Join(in.Calls, "|")
	if !strings.Contains(joined, "key_down 17|key_down 86") {
		t.Fatalf("long text must paste with ctrl+v: %v", in.Calls)
	}
	if clip.Text != "old" {
		t.Fatalf("clipboard must be restored, got %q", clip.Text)
	}
}

// (a) a modifier KeyDown failing partway through a chord must not leave the
// already-pressed modifiers stuck down.
func TestChordReleasesAlreadyPressedModifierOnFailure(t *testing.T) {
	a, in, _ := newActor()
	failer := &failOnNthKeyDown{Input: in, failAt: 2} // 1st KeyDown (Ctrl) ok, 2nd (Shift) fails
	a.In = failer
	c := input.Chord{Mods: []uint16{input.VK_CONTROL, input.VK_SHIFT}, Key: 0x41} // 'A'
	err := a.Chord(c, 0)
	if err == nil {
		t.Fatal("expected error")
	}
	want := "key_down 17|key_up 17"
	if got := strings.Join(in.Calls, "|"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// (b) a MouseMove failing mid-drag (after the button is down) must still release the button.
func TestDragReleasesButtonWhenMoveFailsAfterDown(t *testing.T) {
	a, in, _ := newActor()
	failer := &failOnNthMouseMove{Input: in, failAt: 3} // 1st move (from) ok, then down, then 2nd loop move fails
	a.In = failer
	err := a.Drag(geom.Point{X: 0, Y: 0}, geom.Point{X: 100, Y: 50}, platform.ButtonLeft, 250*time.Millisecond)
	if err == nil {
		t.Fatal("expected error")
	}
	calls := in.Calls
	if len(calls) == 0 || calls[len(calls)-1] != "up left" {
		t.Fatalf("button not released after failed move: %v", calls)
	}
}

// (c) if reading the old clipboard text fails, paste must never clobber the clipboard with "".
func TestPasteDoesNotClobberClipboardWhenGetTextFails(t *testing.T) {
	a, _, clip := newActor()
	a.Clip = &failGetTextClipboard{Clipboard: clip}
	long := strings.Repeat("x", 300)
	if err := a.Type(long, "auto", 0); err != nil {
		t.Fatal(err)
	}
	if clip.Text != long {
		t.Fatalf("clipboard should hold the pasted text (never cleared to \"\"), got %q", clip.Text)
	}
}

// (d) if the Ctrl+V chord fails, the original clipboard text must still be restored.
func TestPasteRestoresClipboardWhenChordFails(t *testing.T) {
	a, in, clip := newActor()
	failer := &failOnNthKeyDown{Input: in, failAt: 2} // 1st KeyDown (Ctrl) ok, 2nd (V) fails
	a.In = failer
	long := strings.Repeat("y", 300)
	err := a.Type(long, "auto", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if clip.Text != "old" {
		t.Fatalf("clipboard must be restored even when the paste chord fails, got %q", clip.Text)
	}
}

func TestScrollMovesFirstWhenPointGiven(t *testing.T) {
	a, in, _ := newActor()
	p := geom.Point{X: 5, Y: 6}
	_ = a.Scroll(&p, 0, 3)
	if strings.Join(in.Calls, "|") != "move 5,6|scroll 0,3" {
		t.Fatalf("got %v", in.Calls)
	}
}

func TestMoveToGlidesWithEasingAndEndsExactly(t *testing.T) {
	a, in, _ := newActor()
	a.GlideMs = 200
	cur := geom.Point{X: 0, Y: 0}
	a.Pos = func() (geom.Point, error) { return cur, nil }
	if err := a.MoveTo(geom.Point{X: 400, Y: 0}); err != nil {
		t.Fatal(err)
	}
	if len(in.Calls) < 12 || in.Calls[len(in.Calls)-1] != "move 400,0" {
		t.Fatalf("glide must emit >=12 moves ending at the target: %v", in.Calls)
	}
	prev := -1
	for _, c := range in.Calls {
		var x, y int
		fmt.Sscanf(c, "move %d,%d", &x, &y)
		if x < prev {
			t.Fatalf("x must be monotonic: %v", in.Calls)
		}
		prev = x
	}
}

func TestPressMovesAndHolds(t *testing.T) {
	a, in, _ := newActor()
	if err := a.Press(geom.Point{X: 50, Y: 60}, platform.ButtonLeft); err != nil {
		t.Fatal(err)
	}
	want := "move 50,60|down left"
	if got := strings.Join(in.Calls, "|"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestReleaseWithoutPointReleasesOnly(t *testing.T) {
	a, in, _ := newActor()
	if err := a.Release(nil, platform.ButtonLeft); err != nil {
		t.Fatal(err)
	}
	want := "up left"
	if got := strings.Join(in.Calls, "|"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestReleaseWithPointMovesFirst(t *testing.T) {
	a, in, _ := newActor()
	p := geom.Point{X: 100, Y: 200}
	if err := a.Release(&p, platform.ButtonLeft); err != nil {
		t.Fatal(err)
	}
	want := "move 100,200|up left"
	if got := strings.Join(in.Calls, "|"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDragViaInterpolatesWaypointsAndReleases(t *testing.T) {
	a, in, _ := newActor()
	from := geom.Point{X: 0, Y: 0}
	wp := Waypoint{P: geom.Point{X: 100, Y: 0}, WaitMs: 50}
	to := geom.Point{X: 200, Y: 0}
	if err := a.DragVia(from, []Waypoint{wp}, to, platform.ButtonLeft, 200*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	calls := in.Calls
	// must start with move to from, then down
	if calls[0] != "move 0,0" || calls[1] != "down left" {
		t.Fatalf("bad start: %v", calls[:3])
	}
	// must end with move to 200,0 then up
	if calls[len(calls)-1] != "up left" || calls[len(calls)-2] != "move 200,0" {
		t.Fatalf("bad end: %v", calls[len(calls)-3:])
	}
	// must have at least 8 intermediate moves to the waypoint
	movesBeforeWP := 0
	for _, c := range calls[2:] {
		if c == "move 100,0" {
			break
		}
		if strings.HasPrefix(c, "move ") {
			movesBeforeWP++
		}
	}
	if movesBeforeWP < 7 { // at least 8 steps including the final one
		t.Fatalf("expected >= 8 interpolated moves to waypoint, got %d moves before it; calls: %v", movesBeforeWP, calls)
	}
}

func TestMoveToTeleportsWhenDisabledOrTiny(t *testing.T) {
	a, in, _ := newActor()
	a.GlideMs = 0
	a.Pos = func() (geom.Point, error) { return geom.Point{}, nil }
	_ = a.MoveTo(geom.Point{X: 300, Y: 300})
	if len(in.Calls) != 1 {
		t.Fatalf("GlideMs=0 must teleport: %v", in.Calls)
	}
	in.Calls = nil
	a.GlideMs = 200
	_ = a.MoveTo(geom.Point{X: 2, Y: 1})
	if len(in.Calls) != 1 {
		t.Fatalf("tiny distance must teleport: %v", in.Calls)
	}
}
