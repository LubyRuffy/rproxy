//go:build !darwin && !linux && !netbsd && !openbsd
// +build !darwin,!linux,!netbsd,!openbsd

package utils

// CheckAndSetUlimit is a no-op on non-unix systems
func CheckAndSetUlimit() error {
	return nil
}
