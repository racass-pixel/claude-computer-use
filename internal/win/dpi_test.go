//go:build windows

package win

import "testing"

func TestCheckRequiredProcsReturnsNilOnWindows10Plus(t *testing.T) {
	if err := CheckRequiredProcs(); err != nil {
		t.Fatalf("CheckRequiredProcs failed on this machine: %v", err)
	}
}
