package contextinfo

import (
	"os"
	"runtime"
)

// detectRuntime gathers host runtime details.
func detectRuntime() RuntimeInfo {
	host, _ := os.Hostname()
	return RuntimeInfo{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: host,
	}
}
