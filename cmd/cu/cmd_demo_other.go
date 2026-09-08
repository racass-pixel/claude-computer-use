//go:build !windows

package main

import "errors"

func runDemo(args []string) error { return errors.New("cu demo is Windows-only") }
