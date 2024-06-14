package logger

import (
	"log/slog"
	"os"
	"path/filepath"
)

var level slog.LevelVar

func replace(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.SourceKey {
		source := a.Value.Any().(*slog.Source)
		source.File = filepath.Base(source.File)
	}
	return a
}

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       &level,
		ReplaceAttr: replace,
	})))
}

func SetLevel(l slog.Level) {
	level.Set(l)
}
