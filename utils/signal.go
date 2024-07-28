package utils

import (
	"log/slog"
	"os"
	"os/signal"
)

func WaitSignal(s ...os.Signal) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, s...)
	sig := <-c
	slog.Info("notify", slog.Any("signal", sig))
}
