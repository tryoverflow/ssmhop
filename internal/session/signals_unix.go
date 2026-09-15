//go:build unix

package session

import (
	"os"
	"os/signal"
	"syscall"
)

func ignoreTerminalSignals() func() {
	sigs := []os.Signal{syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTSTP}
	signal.Ignore(sigs...)
	return func() { signal.Reset(sigs...) }
}
