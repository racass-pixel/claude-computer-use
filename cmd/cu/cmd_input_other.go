//go:build !windows

package main

import "errors"

func runInput(args []string) error { return errors.New("cu input is Windows-only") }
