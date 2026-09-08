package main

import "errors"

var errNotImplemented = errors.New("not implemented yet")

func runServe(args []string) error { return errNotImplemented }
func runCtl(args []string) error   { return errNotImplemented }
func runDemo(args []string) error  { return errNotImplemented }
