package main

import (
	"os"
	"strconv"
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
	var unlock int32
	if err = ioctl(master, syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); err != nil {
		_ = master.Close()
		return nil, nil, err
	}
	var n uint32
	if err = ioctl(master, syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n))); err != nil {
		_ = master.Close()
		return nil, nil, err
	}
	tty, err = os.OpenFile("/dev/pts/"+strconv.Itoa(int(n)), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		_ = master.Close()
		return nil, nil, err
	}
	return master, tty, nil
}
