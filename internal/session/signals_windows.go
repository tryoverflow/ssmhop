package session

import (
	"os"
	"os/signal"
)

func ignoreTerminalSignals() func() {
	sigs := []os.Signal{os.Interrupt}
	signal.Ignore(sigs...)
	return func() { signal.Reset(sigs...) }
}
