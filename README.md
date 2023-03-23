# About

go-win-netcontrol is a utility to allow unprivileged users to enable and disable network access (by enabling or disabling network interfaces) by using a password. This is useful for academic competitions that allow computers but require the Internet to be disabled. This utility has a minimal GUI that allows a teacher or coach to enter a password to enable or disable network access without needing administrator privileges.

# How it Works

go-win-netcontrol is installed as a Windows Service running as the local system account. The service uses WMI to enable or disable all network interfaces. It listens on a unix socket for requests authenticated by an embedded password.

The same executable also runs as the GUI client, which communicates with the server over the unix socket. A teacher or coach enters the password and clicks a button to Enable or Disable network access.

# Changing the Password

The password is embedded in the binary as an Argon2id hash. The default password is "password", which should obviously be changed. To generate a new hash, run:

`HASHPASSWORD="<password>" go test ./hash -v`

Use the hash output when building using the instructions below. You can change the Argon2id parameters (default time=2, memory=64MB, threads=1) by editing `hash/hash_test.go`.

# Building

The easiest way to build go-win-netcontrol is with [fyne-cross](https://github.com/fyne-io/fyne-cross). Run:

`fyne-cross windows -console -ldflags "-X main.passhashstr=<yourhash>"`

The built binary will found in `fyne-cross/bin/windows-amd64/go-win-netcontrol.exe`.

# Installing

Copy the built exe to the device and run (as an administrator):

`.\go-win-netcontrol.exe service install`

This will install and start the service, and place a shortcut to the GUI on the Public Desktop (e.g. for all users).

Service logs can be found in `C:\Program Files\go-win-netcontrol\logs`
