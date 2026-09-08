// Package uithread runs closures on one locked OS thread that pumps a Win32 message loop.
// Overlay windows and low-level hooks must live on such a thread.
package uithread
