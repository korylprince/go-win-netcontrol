package main

import (
	"errors"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var errInvalidPassword = errors.New("invalid password")

func popup(a fyne.App, msg string) {
	win := a.NewWindow("Message")
	win.SetContent(container.NewVBox(
		widget.NewLabel(msg),
		widget.NewButton("Close", func() { win.Close() }),
	),
	)
	win.Show()
}

func updateStatusText(conn *Conn, status binding.String) error {
	all, enabled, err := conn.Count()
	if err != nil {
		return fmt.Errorf("could not get status: %w", err)
	}

	if enabled == 0 {
		err = status.Set("Network Disabled")
	} else if all == enabled {
		err = status.Set("Network Enabled")
	} else {
		err = status.Set("Network Partially Enabled")
	}

	if err != nil {
		return fmt.Errorf("could not update status: %w", err)
	}

	return nil
}

func setStatus(conn *Conn, enabled bool, passwd, status binding.String) error {
	p, err := passwd.Get()
	if err != nil {
		return fmt.Errorf("could not get password: %w", err)
	}

	if !Validate(p) {
		return errInvalidPassword
	}

	if err = NewClient().SetStatus(p, enabled); err != nil {
		return fmt.Errorf("could not set status: %w", err)
	}

	if err = passwd.Set(""); err != nil {
		return fmt.Errorf("could not clear password: %w", err)
	}

	return updateStatusText(conn, status)
}

func runUI() {
	myapp := app.New()
	myapp.Settings().SetTheme(theme.DarkTheme())
	win := myapp.NewWindow("Internet Control")

	conn, err := NewConn()
	if err != nil {
		err = fmt.Errorf("could not create WMI conn: %w", err)
		popup(myapp, err.Error())
		win.ShowAndRun()
		return
	}
	defer conn.Close()

	status := binding.NewString()
	statusLbl := widget.NewLabelWithData(status)
	passwd := binding.NewString()
	passwdEtr := widget.NewEntry()
	passwdEtr.Password = true
	passwdEtr.Wrapping = fyne.TextTruncate
	passwdEtr.Bind(passwd)

	enBtn := widget.NewButton("Enable", func() {
		err := setStatus(conn, true, passwd, status)
		if err == nil {
			popup(myapp, "Network Enabled")
		} else {
			popup(myapp, err.Error())
		}
	})

	disBtn := widget.NewButton("Disable", func() {
		err := setStatus(conn, false, passwd, status)
		if err == nil {
			popup(myapp, "Network Disabled")
		} else {
			popup(myapp, err.Error())
		}
	})

	lblBox := container.NewHBox(layout.NewSpacer(), statusLbl, layout.NewSpacer())
	btnBox := container.NewHBox(layout.NewSpacer(), enBtn, disBtn, layout.NewSpacer())
	vbox := container.NewVBox(lblBox, passwdEtr, btnBox)

	win.SetContent(vbox)

	if err = updateStatusText(conn, status); err != nil {
		popup(myapp, err.Error())
	}

	win.Resize(fyne.NewSize(300, 200))
	win.ShowAndRun()
}
