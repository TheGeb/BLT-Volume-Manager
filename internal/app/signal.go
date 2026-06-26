package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func WithShutdown() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
