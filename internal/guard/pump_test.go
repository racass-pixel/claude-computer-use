package guard

import (
	"testing"
	"time"
)

func TestPumpDrainsInOrder(t *testing.T) {
	p := newPump(64)
	stop := make(chan struct{})
	var got []Event

	done := make(chan struct{})
	go func() {
		p.drain(func(ev Event) { got = append(got, ev) }, stop)
		close(done)
	}()

	const N = 20
	for i := 0; i < N; i++ {
		p.push(Event{VK: uint16(i), At: at(i)})
	}
	// Give the drain goroutine time to process.
	time.Sleep(50 * time.Millisecond)
	close(stop)
	<-done

	if len(got) != N {
		t.Fatalf("received %d events, want %d", len(got), N)
	}
	for i, ev := range got {
		if ev.VK != uint16(i) {
			t.Fatalf("event %d: VK=%d, want %d", i, ev.VK, i)
		}
	}
	if d := p.dropped.Load(); d != 0 {
		t.Fatalf("dropped = %d, want 0", d)
	}
}

func TestPumpDropsOnFull(t *testing.T) {
	p := newPump(4)
	// Fill the channel.
	for i := 0; i < 4; i++ {
		p.push(Event{VK: uint16(i)})
	}
	// This should drop, not block.
	done := make(chan struct{})
	go func() {
		p.push(Event{VK: 99})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("push blocked on a full pump")
	}
	if d := p.dropped.Load(); d != 1 {
		t.Fatalf("dropped = %d, want 1", d)
	}
}
