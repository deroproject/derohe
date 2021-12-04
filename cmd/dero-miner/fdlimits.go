//go:build !windows
// +build !windows

package main

import "runtime"
import "golang.org/x/sys/unix"

// we skip type as go will automatically identify type
const (
	UnixMax = 20
	OSXMax  = 20 // see this https://github.com/golang/go/issues/30401
)

type Limits struct {
	Current uint64
	Max     uint64
}

func init() {
	switch runtime.GOOS {
	case "darwin":
		unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{Max: OSXMax, Cur: OSXMax})
	case "linux", "netbsd", "openbsd", "freebsd":
		unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{Max: UnixMax, Cur: UnixMax})
	default: // nothing to do
	}
}

func Get() (*Limits, error) {
	var rLimit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rLimit); err != nil {
		return nil, err
	}
	return &Limits{Current: uint64(rLimit.Cur), Max: uint64(rLimit.Max)}, nil
}

/*
func Set(maxLimit uint64) error {
	rLimit := unix.Rlimit {Max:maxLimit, Cur:maxLimit}
	if runtime.GOOS == "darwin" && rLimit.Cur > OSXMax { //https://github.com/golang/go/issues/30401
		rLimit.Cur = OSXMax
	}
	return unix.Setrlimit(unix.RLIMIT_NOFILE, &rLimit)
}
*/
