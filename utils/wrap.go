package utils

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

func Wrap(err error) error {
	if err == nil {
		return nil
	}

	if _, file, line, ok := runtime.Caller(1); ok {
		return fmt.Errorf("%s:%d > %w", filepath.Base(file), line, err)
	}

	return err
}

func FileLine(err error) string {
	s := err.Error()

	if n := strings.LastIndex(s, " > "); n != -1 {
		return s[:n]
	}

	return s
}
