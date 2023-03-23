//go:build windows

package main

import (
	"fmt"
	"io"
	stdlog "log"
	"os"

	"github.com/rs/zerolog"
	"golang.org/x/sys/windows"
)

type stdLogger struct {
	zerolog.Logger
	Level zerolog.Level
	Svc   string
}

func (l *stdLogger) Write(p []byte) (n int, err error) {
	n = len(p)
	if n > 0 && p[n-1] == '\n' {
		// Trim CR added by stdlog.
		p = p[0 : n-1]
	}
	l.Logger.WithLevel(l.Level).CallerSkipFrame(2).Str("svc", l.Svc).Msg(string(p))
	return
}

func RedirectOutput(logger zerolog.Logger, f *os.File) error {
	// redirect stdout through logger
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("could not create stdout pipe: %w", err)
	}

	if err := windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, windows.Handle(w.Fd())); err != nil {
		return fmt.Errorf("could not redirect stdout: %w", err)
	}
	os.Stdout = w
	go io.Copy(&stdLogger{Logger: logger, Level: zerolog.InfoLevel, Svc: "stdout"}, r)

	// redirect stderr straight to log file
	if err := windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(f.Fd())); err != nil {
		return fmt.Errorf("could not redirect stderr: %w", err)
	}
	os.Stderr = f

	// redirect std log output
	stdlog.SetFlags(0)
	stdlog.SetOutput(logger.With().Str("svc", "go/log").Logger())

	return nil
}
