//go:build !windows

package uia

import "fmt"

type UIA struct{}

func New() (*UIA, error) { return nil, fmt.Errorf("ui automation is Windows-only") }
func (u *UIA) Close()    {}
