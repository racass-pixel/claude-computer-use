//go:build !windows

package main

import "errors"

func runDoctor(args []string) error { return errors.New("cu doctor is Windows-only") }
