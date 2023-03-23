//go:build windows

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

var CLI struct {
	Service *ServiceCmd `cmd:"" help:"monitor or install monitoring service"`
}

type ServiceCmd struct {
	Status  *StatusServiceCmd  `cmd:"" help:"check service status"`
	Install *InstallServiceCmd `cmd:"" help:"install or reinstall service"`
	Start   *StartServiceCmd   `cmd:"" help:"start service"`
	Stop    *StopServiceCmd    `cmd:"" help:"stop service"`
	Restart *RestartServiceCmd `cmd:"" help:"restart service"`
	Remove  *RemoveServiceCmd  `cmd:"" help:"remove service"`
	Run     *RunServiceCmd     `cmd:"" help:"run in windows service mode."`
}

type StatusServiceCmd struct{}

func (c *StatusServiceCmd) Run() error {
	// check if service is installed
	svcmgr, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("could not connect to service manager: %w", err)
	}
	defer svcmgr.Disconnect()

	svcs, err := svcmgr.ListServices()
	if err != nil {
		return fmt.Errorf("could not get services: %w", err)
	}
	found := false
	for _, s := range svcs {
		if s == ServiceConfig.Name {
			found = true
			break
		}
	}

	stdin := bufio.NewScanner(os.Stdin)

	// install service
	if !found {
		fmt.Print("Service is not installed. Would you like to install it? (y/n): ")
		stdin.Scan()
		if strings.TrimSpace(stdin.Text()) != "y" {
			return nil
		}
		if err = (&InstallServiceCmd{NoStart: true}).Run(); err != nil {
			return err
		}

		// wait for service to start
		time.Sleep(2 * time.Second)
	}
	fmt.Println("Service: Installed")

	// check if service is running
	service, err := svcmgr.OpenService(ServiceConfig.Name)
	if err != nil {
		return fmt.Errorf("could not open service: %w", err)
	}
	defer service.Close()

	status, err := service.Query()
	if err != nil {
		return fmt.Errorf("could not query service: %w", err)
	}

	// start service
	if status.State != svc.Running {
		fmt.Print("Service is not running. Would you like to start it? (y/n): ")
		stdin.Scan()
		if strings.TrimSpace(stdin.Text()) != "y" {
			return nil
		}
		count := 0
		for count < 5 {
			if err = service.Start(); err != nil {
				return fmt.Errorf("could not start service: %w", err)
			}
			time.Sleep(2 * time.Second)
			status, err := service.Query()
			if err != nil {
				return fmt.Errorf("could not query service: %w", err)
			}
			if status.State == svc.Running {
				break
			}
			count++
		}
	}

	fmt.Println("Service: Running")
	return nil
}

type InstallServiceCmd struct {
	NoStart bool `help:"don't start the service after install"`
}

func (c *InstallServiceCmd) Run() error {
	if err := ServiceConfig.Install(!c.NoStart); err != nil {
		return err
	}

	if err := CreateLink(filepath.Join(ServiceConfig.InstallPath, ServiceConfig.ExecName), filepath.Join(`C:\Users\Public\Desktop`, fmt.Sprintf("%s.lnk", ServiceConfig.DisplayName))); err != nil {
		return fmt.Errorf("could not install shortcut: %w", err)
	}

	if c.NoStart {
		return nil
	}

	time.Sleep(2 * time.Second)

	return (&StatusServiceCmd{}).Run()
}

func controlService(c svc.Cmd) error {
	svcmgr, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("could not connect to service manager: %w", err)
	}
	defer svcmgr.Disconnect()

	service, err := svcmgr.OpenService(ServiceConfig.Name)
	if err != nil {
		return fmt.Errorf("could not open service: %w", err)
	}
	defer service.Close()

	_, err = service.Control(c)
	if err != nil {
		return fmt.Errorf("could not sent command: %w", err)
	}

	return nil
}

type StartServiceCmd struct{}

func (c *StartServiceCmd) Run() error {
	svcmgr, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("could not connect to service manager: %w", err)
	}
	defer svcmgr.Disconnect()

	service, err := svcmgr.OpenService(ServiceConfig.Name)
	if err != nil {
		return fmt.Errorf("could not open service: %w", err)
	}
	defer service.Close()

	if err = service.Start(); err != nil {
		return fmt.Errorf("could not start service: %w", err)
	}

	fmt.Println("Service: Starting")
	return nil
}

type StopServiceCmd struct{}

func (c *StopServiceCmd) Run() error {
	if err := controlService(svc.Stop); err != nil {
		return err
	}
	fmt.Println("Service: Stopping")
	return nil
}

type RestartServiceCmd struct{}

func (c *RestartServiceCmd) Run() error {
	if err := (&StopServiceCmd{}).Run(); err != nil {
		if !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) && !errors.Is(err, windows.ERROR_SERVICE_NEVER_STARTED) {
			fmt.Println("WARNING: could not stop service:", err)
		}
	}
	time.Sleep(2 * time.Second)
	return (&StartServiceCmd{}).Run()
}

type RemoveServiceCmd struct{}

func (c *RemoveServiceCmd) Run() error {
	if err := ServiceConfig.Uninstall(); err != nil {
		return err
	}

	fmt.Println("Service is removed. To completely uninstall, remove", ServiceConfig.InstallPath)
	return nil
}
