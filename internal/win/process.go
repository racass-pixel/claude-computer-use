//go:build windows

package win

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procGetUserDefaultUILanguage = kernel32.NewProc("GetUserDefaultUILanguage")

// ParentPID finds the parent of the current process via the toolhelp snapshot.
func ParentPID() (uint32, error) {
	me := windows.GetCurrentProcessId()
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		if pe.ProcessID == me {
			return pe.ParentProcessID, nil
		}
	}
	return 0, fmt.Errorf("parent process not found")
}

// WaitForProcessExit blocks until pid exits.
func WaitForProcessExit(pid uint32) error {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	_, err = windows.WaitForSingleObject(h, windows.INFINITE)
	return err
}

// UserUILanguage maps the user's UI language to "ru" or "en".
func UserUILanguage() string {
	r, _, _ := procGetUserDefaultUILanguage.Call()
	if r&0x3FF == 0x19 { // LANG_RUSSIAN
		return "ru"
	}
	return "en"
}
