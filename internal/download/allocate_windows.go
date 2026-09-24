package download

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

var setFileInformation = syscall.NewLazyDLL("kernel32.dll").NewProc("SetFileInformationByHandle")

// FileAllocationInfo нь файлын disk allocation-ийг урьдчилан нөөцөлнө.
// SetFileValidData ашиглахгүй: хуучин дискний өгөгдөл ил гарах ёсгүй.
func allocate(f *os.File, size int64) error {
	if size > 0 {
		allocation := struct{ Size int64 }{size}
		ok, _, err := setFileInformation.Call(f.Fd(), 5, uintptr(unsafe.Pointer(&allocation)), unsafe.Sizeof(allocation))
		runtime.KeepAlive(f)
		if ok == 0 {
			return os.NewSyscallError("SetFileInformationByHandle", err)
		}
	}
	return f.Truncate(size)
}
