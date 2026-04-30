//go:build windows

package cli

import (
	"os"

	"golang.org/x/sys/windows"
)

func openFileShared(path string) (*os.File, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	access := uint32(windows.GENERIC_READ)
	shareMode := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	attrs := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	h, err := windows.CreateFile(pathPtr, access, shareMode, nil, windows.OPEN_EXISTING, attrs, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}

