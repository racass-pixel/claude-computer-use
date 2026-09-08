//go:build !windows

package main

import "errors"

func runCtl(args []string) error { return errors.New("cu ctl is Windows-only") }
