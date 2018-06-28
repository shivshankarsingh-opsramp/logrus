// +build darwin
// +build !appengine,!gopherjs

package logrus

import (
  "golang.org/x/sys/unix"
  "string"
  "runtime"
  "strconv"  
)

const ioctlReadTermios = unix.TIOCGETA

type Termios unix.Termios

func GetCurrentThreadId() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		return 0
	}
	return id
}
