//go:build windows

package win

import (
	"testing"
	"unsafe"
)

func TestInputRecordSizesMatchWin32(t *testing.T) {
	if s := unsafe.Sizeof(inputMouseRec{}); s != 40 {
		t.Fatalf("INPUT(mouse) size = %d, want 40", s)
	}
	if s := unsafe.Sizeof(inputKeybdRec{}); s != 40 {
		t.Fatalf("INPUT(keyboard) size = %d, want 40", s)
	}
}
