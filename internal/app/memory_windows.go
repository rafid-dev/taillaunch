//go:build windows

package app

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type memoryStatusEx struct {
	Length        uint32
	MemoryLoad    uint32
	TotalPhys     uint64
	AvailPhys     uint64
	TotalPageFile uint64
	AvailPageFile uint64
	TotalVirtual  uint64
	AvailVirtual  uint64
	AvailExtended uint64
}

func totalRAMBytes() (uint64, error) {
	status := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	ret, _, callErr := proc.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 {
		if callErr != windows.ERROR_SUCCESS {
			return 0, callErr
		}
		return 0, fmt.Errorf("GlobalMemoryStatusEx failed")
	}
	return status.TotalPhys, nil
}
