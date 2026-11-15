package main

import (
	"bytes"
	"os"
	"syscall"
	"unsafe"
)

// openPTY is a new pseudo-terminal: its master, which the test reads and writes, and its terminal, which the
// demo runs on.
func openPTY() (master, tty *os.File, err error) {
	master, err = os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	var name [128]byte
	for _, req := range []struct {
		op  uintptr
		arg uintptr
	}{{syscall.TIOCPTYGRANT, 0}, {syscall.TIOCPTYUNLK, 0}, {syscall.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0]))}} {
		if err = ioctl(master, req.op, req.arg); err != nil {
			_ = master.Close()
			return nil, nil, err
		}
	}
	tty, err = os.OpenFile(string(name[:bytes.IndexByte(name[:], 0)]), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		_ = master.Close()
		return nil, nil, err
	}
	return master, tty, nil
}
