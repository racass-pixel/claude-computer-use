//go:build !windows

package main

import "errors"

func runServe(args []string) error { return errors.New("cu serve is Windows-only") }
