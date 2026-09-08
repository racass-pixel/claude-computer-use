package main

import "errors"

var errNotImplemented = errors.New("not implemented yet")

func runCtl(args []string) error  { return errNotImplemented }
func runDemo(args []string) error { return errNotImplemented }
