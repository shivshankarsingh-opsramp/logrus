// +build freebsd openbsd netbsd dragonfly
// +build !appengine,!gopherjs

package logrus

import "golang.org/x/sys/unix"

Import “syscall”

const ioctlReadTermios = unix.TIOCGETA

type Termios unix.Termios

func GetCurrentThreadId() int {
	return syscall.Gettid()
}
