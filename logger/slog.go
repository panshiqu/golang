package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

var level slog.LevelVar

func replace(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
		a.Value = slog.StringValue(a.Value.Time().Format("01-02T15:04:05"))
	}
	if a.Key == slog.SourceKey {
		source := a.Value.Any().(*slog.Source)
		source.File = filepath.Base(source.File)
	}
	return a
}

func Init(l ...slog.Level) {
	if len(l) > 0 {
		level.Set(l[0])
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       &level,
		ReplaceAttr: replace,
	})))

	slog.Info("init", slog.Time("now", time.Now()))
}

func SetLevel(l slog.Level) {
	level.Set(l)
}
