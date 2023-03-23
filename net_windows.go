package main

import (
	"fmt"

	"github.com/korylprince/go-win-netcontrol/wmi"
)

const netAdapterNamespace = `root\StandardCimv2`

const (
	ndisMedium8023        = 0
	ndisMediumNative80211 = 16
)

const (
	interfaceAdminStatusUp   = 1
	interfaceAdminStatusDown = 2
)

const netAdapterQuery = "SELECT Name, InterfaceAdminStatus FROM MSFT_NetAdapter WHERE (NdisMedium = 0 OR NdisMedium = 16) AND Virtual = 0"

// Conn is a WMI conn to query the MSFT_NetAdapter class
type Conn struct {
	conn *wmi.Conn
}

// NewConn returns a new Conn
func NewConn() (*Conn, error) {
	conn, err := wmi.Dial(netAdapterNamespace)
	if err != nil {
		return nil, err
	}
	return &Conn{conn: conn}, nil
}

// Close closes the conn
func (conn *Conn) Close() error {
	return conn.conn.Close()
}

// Count returns the number of all and enabled network interfaces
func (conn *Conn) Count() (all, enabled int, err error) {
	rows, err := conn.conn.Query(netAdapterQuery)
	if err != nil {
		return 0, 0, fmt.Errorf("could not query net adapters: %w", err)
	}
	defer rows.Close()

	all = int(rows.Count)
	count := 0

	for item, err := rows.Next(); err == nil; item, err = rows.Next() {
		name := ""
		nameProp, err := item.GetProperty("Name")
		if err == nil {
			name = nameProp.ToString()
			if err = nameProp.Clear(); err != nil {
				fmt.Println("WARN: could not clear name property:", err)
			}
		}

		statusProp, err := item.GetProperty("InterfaceAdminStatus")
		if err != nil {
			return 0, 0, fmt.Errorf("could not get (%s).InterfaceAdminStatus property: %w", name, err)
		}
		status := statusProp.Value().(int32)
		if err = statusProp.Clear(); err != nil {
			return 0, 0, fmt.Errorf("could not clear (%s).InterfaceAdminStatus property: %w", name, err)
		}

		if status == interfaceAdminStatusUp {
			count += 1
		}
	}
	if err = rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("could not finish iterating rows: %w", err)
	}

	return all, count, nil
}

// SetStatus sets all network interfaces to enabled or disabled
func (conn *Conn) SetStatus(enabled bool) error {
	rows, err := conn.conn.Query(netAdapterQuery)
	if err != nil {
		return fmt.Errorf("could not query net adapters: %w", err)
	}
	defer rows.Close()

	for item, err := rows.Next(); err == nil; item, err = rows.Next() {
		name := ""
		nameProp, err := item.GetProperty("Name")
		if err == nil {
			name = nameProp.ToString()
			if err = nameProp.Clear(); err != nil {
				fmt.Println("WARN: could not clear name property:", err)
			}
		}

		statusProp, err := item.GetProperty("InterfaceAdminStatus")
		if err != nil {
			return fmt.Errorf("could not get %s InterfaceAdminStatus property: %w", name, err)
		}
		status := statusProp.Value().(int32)
		if err = statusProp.Clear(); err != nil {
			return fmt.Errorf("could not clear %s InterfaceAdminStatus property: %w", name, err)
		}

		if status != interfaceAdminStatusUp && enabled {
			_, err := item.CallMethod("Enable")
			if err != nil {
				return fmt.Errorf("could not enable %s: %w", name, err)
			}
		} else if status == interfaceAdminStatusUp && !enabled {
			_, err := item.CallMethod("Disable")
			if err != nil {
				return fmt.Errorf("could not disable %s: %w", name, err)
			}
		}
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("could not finish iterating rows: %w", err)
	}
	if err = rows.Close(); err != nil {
		return fmt.Errorf("could not close rows: %w", err)
	}

	return nil
}
