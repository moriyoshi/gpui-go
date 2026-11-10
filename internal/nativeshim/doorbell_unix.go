//go:build gpui_prebuilt && !windows

package nativeshim

import "syscall"

// dupDoorbell returns a nonblocking copy so reads park in Go's netpoller.
func dupDoorbell(descriptor int) (uintptr, error) {
	duplicate, err := syscall.Dup(descriptor)
	if err != nil {
		return 0, err
	}
	if err := syscall.SetNonblock(duplicate, true); err != nil {
		syscall.Close(duplicate)
		return 0, err
	}
	return uintptr(duplicate), nil
}
