//go:build darwin || linux || netbsd || openbsd
// +build darwin linux netbsd openbsd

package utils

import (
	"fmt"
	"log"
	"runtime"
	"syscall"
)

const fileDescriptorLimit uint64 = 32000

// CheckAndSetUlimit raises the file descriptor limit
func CheckAndSetUlimit() error {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return fmt.Errorf("error getting rlimit: %s", err)
	}

	var setting bool
	oldMax := rLimit.Max

	var oldCur uint64
	if rLimit.Cur < fileDescriptorLimit {
		if rLimit.Max < fileDescriptorLimit {
			rLimit.Max = fileDescriptorLimit
		}
		oldCur = rLimit.Cur
		rLimit.Cur = fileDescriptorLimit
		setting = true
	}

	// If we're on darwin, work around the fact that Getrlimit reports
	// the wrong value. See https://github.com/golang/go/issues/30401
	if runtime.GOOS == "darwin" && rLimit.Cur > 10240 {
		// The max file limit is 10240, even though
		// the max returned by Getrlimit is 1<<63-1.
		// This is OPEN_MAX in sys/syslimits.h.
		rLimit.Max = 10240
		rLimit.Cur = 10240

		if oldCur == rLimit.Cur {
			setting = false
		}
	}

	if !setting {
		//log.Println("Did not change ulimit")
		return nil
	}

	// Try updating the limit. If it fails, try using the previous maximum instead
	// of our new maximum. Not all users have permissions to increase the maximum.
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		rLimit.Max = oldMax
		rLimit.Cur = oldMax
		err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
		if err != nil {
			return fmt.Errorf("error setting ulimit: %s", err)
		}
	}

	log.Println("Successfully raised file descriptor limit to", rLimit.Cur)
	return nil
}
