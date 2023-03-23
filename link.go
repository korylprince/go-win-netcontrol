package main

import (
	"fmt"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

func CreateLink(src, dst string) error {
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		return fmt.Errorf("could not coinitialize: %w", err)
	}

	shell, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return fmt.Errorf("could not create WScript.Shell: %w", err)
	}
	defer shell.Release()

	wshell, err := shell.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("could no create WMI shell: %w", err)
	}
	defer wshell.Release()

	mthdDispatch, err := wshell.CallMethod("CreateShortcut", dst)
	if err != nil {
		return fmt.Errorf("could not call CreateShortcut: %w", err)
	}
	defer mthdDispatch.Clear()

	mthd := mthdDispatch.ToIDispatch()
	if _, err = mthd.PutProperty("TargetPath", src); err != nil {
		return fmt.Errorf("could not set target path: %w", err)
	}

	if _, err = mthd.CallMethod("Save"); err != nil {
		return fmt.Errorf("could not save link: %w", err)
	}
	return nil
}
