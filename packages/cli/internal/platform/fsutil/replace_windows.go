//go:build windows

package fsutil

import (
	"errors"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const replaceRetryWindow = 2 * time.Second

const replaceFileWriteThrough = 0x2

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

func replaceFile(source, target string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(replaceRetryWindow)
	for {
		_, statErr := os.Stat(target)
		if os.IsNotExist(statErr) {
			err = windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH)
		} else if statErr != nil {
			err = statErr
		} else {
			err = callReplaceFile(to, from)
		}
		if err == nil {
			return nil
		}
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return err
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func callReplaceFile(target, replacement *uint16) error {
	result, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(target)),
		uintptr(unsafe.Pointer(replacement)),
		0,
		replaceFileWriteThrough,
		0,
		0,
	)
	if result != 0 {
		return nil
	}
	return callErr
}

func readFile(path string) ([]byte, error) {
	deadline := time.Now().Add(replaceRetryWindow)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func syncDir(string) error { return nil }
