package guard

import "sync/atomic"

// pump is a non-blocking event channel that drops the oldest events when full.
type pump struct {
	ch      chan Event
	dropped atomic.Uint64
}

func newPump(size int) *pump {
	return &pump{ch: make(chan Event, size)}
}

// push sends ev into the channel without blocking. If the channel is full,
// the event is dropped and the dropped counter increments.
func (p *pump) push(ev Event) {
	select {
	case p.ch <- ev:
	default:
		p.dropped.Add(1)
	}
}

// drain reads events from the channel and calls handle for each one.
// It returns when stop is closed.
func (p *pump) drain(handle func(Event), stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case ev := <-p.ch:
			handle(ev)
		}
	}
}
