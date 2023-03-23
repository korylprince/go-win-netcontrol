//go:build windows

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	gosvc "github.com/judwhite/go-svc"
	"github.com/korylprince/go-win-netcontrol/svc"
	"github.com/rs/zerolog"
)

var DefaultLogLevel = zerolog.InfoLevel

var ServiceConfig = &svc.ServiceConfig{
	InstallPath:      `C:\Program Files\go-win-netcontrol`,
	ExecName:         "netcontrol.exe",
	Args:             []string{"service", "run"},
	LogDirName:       "logs",
	Name:             "go-win-netcontrol",
	DisplayName:      "Network Control",
	DelayedAutoStart: false,
	AutoRecovery:     true,
}

type RunServiceCmd struct {
	FG bool `help:"run service in foreground"`
}

func (c *RunServiceCmd) Run() error {
	if c.FG {
		return c.RunFG()
	}
	// open log file
	var err error
	f, err := os.OpenFile(
		filepath.Join(ServiceConfig.InstallPath, ServiceConfig.LogDirName, fmt.Sprintf("%s.log", ServiceConfig.Name)),
		os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644,
	)
	if err != nil {
		return fmt.Errorf("could not open log file: %w", err)
	}
	defer f.Close()

	// set up logger
	logger := zerolog.New(f).Level(DefaultLogLevel).With().Timestamp().Logger()
	logger.Info().Msg("logger started")

	// turn on line logging
	if logger.GetLevel() == zerolog.TraceLevel {
		logger = logger.With().Caller().Logger()
	}

	// windows service main
	var server *Server
	svclogger := logger.With().Str("svc", "windows").Logger()
	main := func() error {
		svclogger.Info().Msg("started")

		// redirect stdout/stderr in case of panics
		if err := RedirectOutput(logger, f); err != nil {
			svclogger.Error().Err(err).Msg("could not redirect stdout/stderr")
		}

		// start server
		server, err = NewServer(logger.With().Str("svc", "http").Logger())
		if err != nil {
			svclogger.Error().Err(err).Msg("could not start server")
			return nil
		}
		if err = server.Serve(); err != nil {
			svclogger.Error().Err(err).Msg("server failed")
		}

		return nil
	}

	// make sure server is stopped before windows service is stopped
	stop := func() error {
		defer f.Sync()
		if server != nil {
			err := server.Shutdown()
			return err
		}
		return nil
	}

	if err := gosvc.Run(ServiceConfig.Service(svclogger, main, stop)); err != nil {
		if err == svc.ErrNotWindowsService {
			fmt.Println("This command is intended to be run as a windows service. Use the --fg flag to run directly.")
			return nil
		}

		svclogger.Error().Err(err).Msg("service failed")
		return nil
	}

	return nil
}

func (c *RunServiceCmd) RunFG() error {
	// set up logger
	logger := zerolog.New(os.Stdout).Level(DefaultLogLevel).With().Timestamp().Logger()
	logger.Info().Msg("logger started")

	// turn advanced trace logging
	if logger.GetLevel() == zerolog.TraceLevel {
		logger = logger.With().Caller().Logger()
		// enable http/pprof
		if addr := os.Getenv("PPROF_ADDR"); addr != "" {
			go func() {
				logger.Info().Str("svc", "pprof").Msg("started")
				if err := http.ListenAndServe(addr, nil); err != nil {
					logger.Error().Err(err).Msg("failed")
				}
			}()
		}
	}

	// start server
	server, err := NewServer(logger.With().Str("svc", "http").Logger())
	if err != nil {
		logger.Error().Err(err).Msg("could not start server")
		return nil
	}
	if err = server.Serve(); err != nil {
		logger.Error().Err(err).Msg("server failed")
	}
	return nil
}
