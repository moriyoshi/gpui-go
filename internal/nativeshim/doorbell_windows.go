//go:build gpui_prebuilt

package nativeshim

import "syscall"

// dupDoorbell copies the anonymous-pipe read HANDLE, which Sable narrows to a
// C int. As in Sable's own Go binding, the pump blocking-reads the copy.
func dupDoorbell(descriptor int) (uintptr, error) {
	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0, err
	}
	var duplicate syscall.Handle
	if err := syscall.DuplicateHandle(process, syscall.Handle(descriptor), process, &duplicate, 0, false, syscall.DUPLICATE_SAME_ACCESS); err != nil {
		return 0, err
	}
	return uintptr(duplicate), nil
}
