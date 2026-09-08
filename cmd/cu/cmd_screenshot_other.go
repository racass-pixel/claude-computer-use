//go:build !windows

package main

import "errors"

func runScreenshot(args []string) error { return errors.New("cu screenshot is Windows-only") }
