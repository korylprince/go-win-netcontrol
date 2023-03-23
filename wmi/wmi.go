package wmi

import (
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// ErrNilObject indicates an nil object was unexpectedly received
var ErrNilObject = errors.New("returned nil object")

// Conn is a WMI connection. A Conn should only be used by a single thread
type Conn struct {
	locator *ole.IUnknown
	wmi     *ole.IDispatch
	service *ole.VARIANT
}

// Dial returns a new Conn using the given WMI namespace
func Dial(namespace string) (*Conn, error) {
	// attempt to prevent WMI memory issues
	runtime.LockOSThread()

	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		return nil, fmt.Errorf("could not coinitialize: %w", err)
	}

	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return nil, fmt.Errorf("could not create WbemScripting.SWbemLocator: %w", err)
	} else if locator == nil {
		return nil, fmt.Errorf("could not create WbemScripting.SWbemLocator: %w", ErrNilObject)
	}

	wmi, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("could not create WMI object: %w", err)
	} else if wmi == nil {
		return nil, fmt.Errorf("could not create WMI object: %w", ErrNilObject)
	}

	service, err := wmi.CallMethod("ConnectServer", nil, namespace)
	if err != nil {
		return nil, fmt.Errorf("could not connect: %w", err)
	} else if wmi == nil {
		return nil, fmt.Errorf("could not connect: %w", ErrNilObject)
	}

	return &Conn{locator: locator, wmi: wmi, service: service}, nil
}

// Close closes the connection
func (conn *Conn) Close() error {
	if conn.service != nil {
		if err := conn.service.Clear(); err != nil {
			return fmt.Errorf("could not clear service: %w", err)
		}
	}

	if conn.wmi != nil {
		conn.wmi.Release()
	}

	if conn.locator != nil {
		conn.locator.Release()
	}

	ole.CoUninitialize()

	runtime.UnlockOSThread()

	return nil
}

// Rows is the result of a query
type Rows struct {
	Count    int32
	result   *ole.VARIANT
	enumProp *ole.VARIANT
	enum     *ole.IEnumVARIANT
	item     *ole.IDispatch
	err      error
}

// Close closes the rows and should be always be called on returned Rows
func (rows *Rows) Close() error {
	if rows.item != nil {
		rows.item.Release()
	}

	if rows.enum != nil {
		rows.enum.Release()
	}

	if rows.enumProp != nil {
		if err := rows.enumProp.Clear(); err != nil {
			return fmt.Errorf("could not clear enum: %w", err)
		}
	}

	if rows.result != nil {
		if err := rows.result.Clear(); err != nil {
			return fmt.Errorf("could not clear result: %w", err)
		}
	}

	return nil
}

// Query executes the query and returns a rows cursor
func (conn *Conn) Query(query string) (*Rows, error) {
	result, err := conn.service.ToIDispatch().CallMethod("ExecQuery", query)
	if err != nil {
		return nil, fmt.Errorf("could not execute query: %w", err)
	}
	resDispatch := result.ToIDispatch()

	// force query error to materialize
	countProp, err := resDispatch.GetProperty("Count")
	if err != nil {
		return nil, fmt.Errorf("could not get query results: %w", err)
	}
	count := countProp.Value().(int32)
	if err = countProp.Clear(); err != nil {
		return nil, fmt.Errorf("could not clear count property: %w", err)
	}

	// create enumerator
	enumProp, err := resDispatch.GetProperty("_NewEnum")
	if err != nil {
		return nil, fmt.Errorf("could not get enumerator property: %w", err)
	}

	enum, err := enumProp.ToIUnknown().IEnumVARIANT(ole.IID_IEnumVariant)
	if err != nil {
		return nil, fmt.Errorf("could not convert enumerator to IEnumVariant: %w", err)
	}
	if enum == nil {
		return nil, fmt.Errorf("could not convert enumerator to IEnumVariant: %w", ErrNilObject)
	}

	return &Rows{Count: count, result: result, enumProp: enumProp, enum: enum}, nil
}

// Next returns the next row, which is only valid until the next call to Next. io.EOF is returned at the end of iteration
func (rows *Rows) Next() (*ole.IDispatch, error) {
	if rows.item != nil {
		rows.item.Release()
		rows.item = nil
	}

	item, length, err := rows.enum.Next(1)
	if length == 0 {
		return nil, io.EOF
	}
	if err != nil {
		rows.err = fmt.Errorf("could not get next row: %w", err)
		return nil, rows.err
	}
	rows.item = (&item).ToIDispatch()

	return rows.item, nil
}

// Err returns the last error returned by Next, unless it was io.EOF
func (rows *Rows) Err() error {
	return rows.err
}
