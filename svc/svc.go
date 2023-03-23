//go:build windows

package svc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/judwhite/go-svc"
	"github.com/korylprince/go-win-netcontrol/retry"
	"github.com/rs/zerolog"
	"golang.org/x/exp/slices"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	wsvc "golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// ErrNotWindowsService indicates the process was not started as a windows service
var ErrNotWindowsService = errors.New("process not started as Windows service")

// consts to broadcast an environment variable change
const (
	PathRegKey       = "Path"
	HWND_BROADCAST   = uintptr(0xffff)
	WM_SETTINGCHANGE = uintptr(0x001A)
)

// ServiceConfig holds information to create a Windows service
type ServiceConfig struct {
	InstallPath      string
	ExecName         string
	Args             []string
	LogDirName       string
	Name             string
	DisplayName      string
	Description      string
	DelayedAutoStart bool
	AutoRecovery     bool
	AddToPath        bool
}

// Install installs the service executable, creates the Windows service, and starts it
func (s *ServiceConfig) Install(start bool) error {
	// create service directory
	if err := os.MkdirAll(s.InstallPath, 0644); err != nil {
		return fmt.Errorf("could not create service executable directory: %w", err)
	}

	// create log directory
	if err := os.MkdirAll(filepath.Join(s.InstallPath, s.LogDirName), 0644); err != nil {
		return fmt.Errorf("could not create service log directory: %w", err)
	}

	// connext to service manager
	svcmgr, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("could not connect to service manager: %w", err)
	}
	defer svcmgr.Disconnect()

	// stop and delete service
	service, err := svcmgr.OpenService(s.Name)
	if err != nil {
		if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			fmt.Println("WARN: could not find service:", err)
		}
	} else {
		if _, err = service.Control(wsvc.Stop); err != nil {
			if !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) && !errors.Is(err, windows.ERROR_SERVICE_NEVER_STARTED) {
				fmt.Println("WARN: could not stop service:", err)
			}
		}
		if err = service.Delete(); err != nil {
			fmt.Println("WARN: could not delete service:", err)
		}
		service.Close()
	}

	// wait for service to be deleted
	now := time.Now()
	for {
		if time.Since(now) > 10*time.Second {
			return errors.New("service couldn't be deleted: timeout waiting for deletion (check if services.msc is open)")
		}
		if service, err = svcmgr.OpenService(s.Name); err == nil {
			service.Close()
		} else if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			break
		} else {
			return fmt.Errorf("service couldn't be deleted: %w", err)
		}
		time.Sleep(time.Second)
	}

	// copy self to service directory
	curpath, err := os.Executable()
	execpath := filepath.Join(s.InstallPath, s.ExecName)
	if err != nil {
		return fmt.Errorf("could not get path to current executable: %w", err)
	}
	if filepath.Clean(curpath) != filepath.Clean(execpath) {
		r, err := os.Open(curpath)
		if err != nil {
			return fmt.Errorf("could not read service executable: %w", err)
		}
		w, err := os.Create(execpath)
		if err != nil {
			r.Close()
			return fmt.Errorf("could not create service executable: %w", err)
		}
		if _, err = w.ReadFrom(r); err != nil {
			r.Close()
			w.Close()
			return fmt.Errorf("could not copy service executable: %w", err)
		}
		if err = w.Close(); err != nil {
			return fmt.Errorf("could not close service executable: %w", err)
		}
		r.Close()
	}

	// create service config
	config := mgr.Config{
		ServiceType:      windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:        mgr.StartAutomatic,
		ErrorControl:     windows.SERVICE_ERROR_NORMAL,
		ServiceStartName: "LocalSystem",
		DisplayName:      s.DisplayName,
		Description:      s.Description,
		DelayedAutoStart: s.DelayedAutoStart,
	}

	// create service
	service, err = svcmgr.CreateService(s.Name, execpath, config, s.Args...)
	if err != nil {
		return fmt.Errorf("could not create service: %w", err)
	}
	defer service.Close()

	// set auto recovery settings
	if s.AutoRecovery {
		if err = service.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: time.Minute}}, 3600); err != nil {
			return fmt.Errorf("could not start configure recovery: %w", err)
		}
	}

	// add to PATH
	if s.AddToPath {
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("could not open registry key: %w", err)
		}
		defer key.Close()

		path, _, err := key.GetStringValue(PathRegKey)
		if err != nil {
			return fmt.Errorf("could not read PATH: %w", err)
		}
		if slices.Contains(strings.Split(path, string(os.PathListSeparator)), s.InstallPath) {
			return nil
		}
		path += string(os.PathListSeparator) + s.InstallPath

		if err = key.SetExpandStringValue(PathRegKey, path); err != nil {
			return fmt.Errorf("could not set PATH: %w", err)
		}

		// signal environment change
		syscall.NewLazyDLL("user32.dll").NewProc("SendMessageW").Call(HWND_BROADCAST, WM_SETTINGCHANGE, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Environment"))))
	}

	if !start {
		return nil
	}

	// start service
	if err = service.Start(); err != nil {
		return fmt.Errorf("could not start service: %w", err)
	}

	return nil
}

// Uninstall uninstalls the service
func (s *ServiceConfig) Uninstall() error {
	// connext to service manager
	svcmgr, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("could not connect to service manager: %w", err)
	}
	defer svcmgr.Disconnect()

	// stop and delete service
	service, err := svcmgr.OpenService(s.Name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return nil
		}
		return fmt.Errorf("could not open service: %w", err)
	}
	defer service.Close()

	if _, err = service.Control(wsvc.Stop); err != nil {
		if !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) && !errors.Is(err, windows.ERROR_SERVICE_NEVER_STARTED) {
			return fmt.Errorf("could not stop service: %w", err)
		}
	}

	if err = service.Delete(); err != nil {
		return fmt.Errorf("could not delete service: %w", err)
	}

	return nil
}

// Service returns a new Service for use with svc.Run
func (s *ServiceConfig) Service(logger zerolog.Logger, main func() error, stop func() error) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{main: main, stop: stop, l: logger, ctx: ctx, cancel: cancel}
}

// Service implements svc.Service
type Service struct {
	main   func() error
	stop   func() error
	l      zerolog.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

// Context implements svc.Context
func (s *Service) Context() context.Context {
	return s.ctx
}

// Init implements svc.Service
func (s *Service) Init(env svc.Environment) error {
	if !env.IsWindowsService() {
		return ErrNotWindowsService
	}

	return nil
}

// Start implements svc.Service
func (s *Service) Start() error {
	s.l.Info().Msg("starting")
	go func() {
		if err := retry.DefaultStrategy.Retry(func() error {
			return s.main()
		}); err != nil {
			s.l.Error().Err(err).Msg("service retries exhausted")
		}
		s.cancel()
	}()
	return nil
}

// Stop implements svc.Service
func (s *Service) Stop() error {
	s.l.Info().Msg("stopping")
	err := s.stop()
	if err != nil {
		s.l.Error().Err(err).Msg("stopped")
		return err
	}
	s.l.Info().Msg("stopped")
	return nil
}
