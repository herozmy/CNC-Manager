//go:build !windows

package main

import (
	"fmt"
	"net"
	"os"
)

func attachCLIConsole()                                 {}
func reportStartupError(err error)                      { fmt.Fprintln(os.Stderr, err) }
func startDesktop(*net.TCPAddr, func()) (func(), error) { return func() {}, nil }
