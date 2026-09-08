//go:build windows

package uithread

import (
	"log"
	"runtime"
	"sync"

	"golang.org/x/sys/windows"

	"github.com/racass-pixel/claude-computer-use/internal/win"
)

const className = "CuDispatch"

type Thread struct {
	hwnd uintptr
	tid  uint32
	q    chan func()
	done chan struct{}
}

var (
	regMu   sync.Mutex
	threads = map[uintptr]*Thread{}
	wndProc = windows.NewCallback(func(hwnd, msg, wparam, lparam uintptr) uintptr {
		if msg == win.WM_APP {
			regMu.Lock()
			t := threads[hwnd]
			regMu.Unlock()
			if t != nil {
				t.drain()
			}
			return 0
		}
		return win.DefWindowProc(hwnd, msg, wparam, lparam)
	})
)

func New() (*Thread, error) {
	t := &Thread{q: make(chan func(), 1024), done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		if err := win.RegisterClass(className, wndProc); err != nil {
			ready <- err
			return
		}
		hwnd, err := win.CreateMessageWindow(className)
		if err != nil {
			ready <- err
			return
		}
		t.hwnd, t.tid = hwnd, win.CurrentThreadID()
		regMu.Lock()
		threads[hwnd] = t
		regMu.Unlock()
		ready <- nil
		win.RunMessageLoop()
		regMu.Lock()
		delete(threads, hwnd)
		regMu.Unlock()
		close(t.done)
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Thread) drain() {
	for {
		select {
		case f := <-t.q:
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("uithread: panic in closure: %v", r)
					}
				}()
				f()
			}()
		default:
			return
		}
	}
}

func (t *Thread) Do(f func()) {
	t.q <- f
	_ = win.PostMessage(t.hwnd, win.WM_APP, 0, 0)
}

func (t *Thread) DoSync(f func()) {
	done := make(chan struct{})
	t.Do(func() { defer close(done); f() })
	<-done
}

func (t *Thread) ID() uint32 { return t.tid }

func (t *Thread) Close() {
	t.Do(win.PostQuitMessage)
	<-t.done
}
