//go:build windows

package gxserial

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"github.com/Gurux/gxcommon-go"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type port struct {
	h       windows.Handle
	ovRead  windows.Overlapped
	ovWrite windows.Overlapped
	closing windows.Handle
}

func (p *port) isOpen() bool {
	return p != nil && p.h != 0 && p.h != windows.InvalidHandle
}

// getPortNames retrieves the list of available serial port names on a Windows system by querying the registry.
func getPortNames() ([]string, error) {
	const path = `HARDWARE\DEVICEMAP\SERIALCOMM`

	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return []string{}, nil
		}
		return nil, err
	}
	defer func() {
		_ = key.Close()
	}()

	valueNames, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, err
	}

	var ports []string
	for _, name := range valueNames {
		port, _, err := key.GetStringValue(name)
		if err == nil {
			ports = append(ports, port)
		}
	}
	return ports, nil
}

const (
	dcbFBinary         = 1 << 0
	dcbFParity         = 1 << 1
	dcbFErrorChar      = 1 << 10
	dcbFNull           = 1 << 11
	dcbFAbortOnError   = 1 << 14
	dcbFDtrControlMask = 0x3 << 4  // bits 4-5
	dcbFRtsControlMask = 0x3 << 12 // bits 12-13
)

// XON/XOFF control characters
const (
	xon  byte = 0x11
	xoff byte = 0x13
)

// RTS/DTR control values (DCB 2-bit fields)
const (
	rtsControlDisable uint32 = 0
	dtrControlDisable uint32 = 0
)

func setBinary(d *windows.DCB, on bool) {
	if on {
		d.Flags |= dcbFBinary
	} else {
		d.Flags &^= dcbFBinary
	}
}
func setParityCheck(d *windows.DCB, on bool) {
	if on {
		d.Flags |= dcbFParity
	} else {
		d.Flags &^= dcbFParity
	}
}
func setNull(d *windows.DCB, on bool) {
	if on {
		d.Flags |= dcbFNull
	} else {
		d.Flags &^= dcbFNull
	}
}
func setErrorChar(d *windows.DCB, on bool) {
	if on {
		d.Flags |= dcbFErrorChar
	} else {
		d.Flags &^= dcbFErrorChar
	}
}
func setAbortOnError(d *windows.DCB, on bool) {
	if on {
		d.Flags |= dcbFAbortOnError
	} else {
		d.Flags &^= dcbFAbortOnError
	}
}
func setRtsControl(d *windows.DCB, val uint32) {
	d.Flags &^= dcbFRtsControlMask
	d.Flags |= (val & 0x3) << 12
}
func setDtrControl(d *windows.DCB, val uint32) {
	d.Flags &^= dcbFDtrControlMask
	d.Flags |= (val & 0x3) << 4
}

func (p *port) getCommState() (*windows.DCB, error) {
	if !p.isOpen() {
		return nil, errors.New("serial port is not open")
	}
	var d windows.DCB
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if err := windows.GetCommState(p.h, &d); err != nil {
		return nil, fmt.Errorf("GetCommState failed: %w", err)
	}
	return &d, nil
}

func (p *port) setCommState(d *windows.DCB) error {
	if !p.isOpen() {
		return errors.New("serial port is not open")
	}
	if err := windows.SetCommState(p.h, d); err != nil {
		return fmt.Errorf("SetCommState failed: %w", err)
	}
	return nil
}

func (p *port) updateSettings(cfg *GXSerial) error {
	d, err := p.getCommState()
	if err != nil {
		return err
	}

	d.BaudRate = uint32(cfg.baudRate)
	d.ByteSize = byte(cfg.dataBits)
	d.Parity = byte(cfg.parity)

	switch cfg.stopBits {
	case gxcommon.StopBitsOne:
		d.StopBits = 0 // ONESTOPBIT
	case gxcommon.StopBitsTwo:
		d.StopBits = 2 // TWOSTOPBITS
	default:
		return gxcommon.ErrInvalidArgument
	}
	setParityCheck(d, d.Parity != 0)
	setBinary(d, true)
	setNull(d, false)
	setErrorChar(d, false)
	setAbortOnError(d, false)
	d.XonChar = xon
	d.XoffChar = xoff
	setRtsControl(d, rtsControlDisable)
	setDtrControl(d, dtrControlDisable)
	return p.setCommState(d)
}

func (p *port) setBaudRate(value gxcommon.BaudRate) error {
	d, err := p.getCommState()
	if err != nil {
		return err
	}
	d.BaudRate = uint32(value)
	return p.setCommState(d)
}

func (p *port) setDataBits(value int) error {
	d, err := p.getCommState()
	if err != nil {
		return err
	}
	d.ByteSize = byte(value)
	return p.setCommState(d)
}

func (p *port) setStopBits(value gxcommon.StopBits) error {
	d, err := p.getCommState()
	if err != nil {
		return err
	}
	switch value {
	case gxcommon.StopBitsOne:
		d.StopBits = 0
	case gxcommon.StopBitsTwo:
		d.StopBits = 2
	default:
		return gxcommon.ErrInvalidArgument
	}
	return p.setCommState(d)
}

func (p *port) setParity(value gxcommon.Parity) error {
	d, err := p.getCommState()
	if err != nil {
		return err
	}
	d.Parity = byte(value)
	return p.setCommState(d)
}

func openPort(cfg *GXSerial) error {
	if strings.TrimSpace(cfg.Port) == "" {
		return errors.New("invalid serial port name")
	}

	cfg.s = port{}

	closing, err := windows.CreateEvent(nil, 1, 1, nil) // manual-reset=TRUE, initial=TRUE
	if err != nil {
		return fmt.Errorf("CreateEvent(closing) failed: %w", err)
	}
	cfg.s.closing = closing

	path := `\\.\` + cfg.Port
	h, err := windows.CreateFile(
		windows.StringToUTF16Ptr(path),
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OVERLAPPED,
		0,
	)
	if err != nil {
		_ = cfg.s.close()
		return fmt.Errorf("failed to open port %q: %w", cfg.Port, err)
	}
	cfg.s.h = h

	er, err := windows.CreateEvent(nil, 0, 0, nil) // auto-reset
	if err != nil {
		_ = cfg.s.close()
		return fmt.Errorf("CreateEvent(read) failed: %w", err)
	}
	cfg.s.ovRead.HEvent = er

	ew, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		_ = cfg.s.close()
		return fmt.Errorf("CreateEvent(write) failed: %w", err)
	}
	cfg.s.ovWrite.HEvent = ew

	if err := windows.ResetEvent(cfg.s.closing); err != nil {
		_ = cfg.s.close()
		return fmt.Errorf("ResetEvent(closing) failed: %w", err)
	}

	if err := cfg.s.updateSettings(cfg); err != nil {
		_ = cfg.s.close()
		return fmt.Errorf("failed to update serial port settings: %w", err)
	}

	if err := windows.PurgeComm(cfg.s.h,
		windows.PURGE_TXCLEAR|windows.PURGE_TXABORT|windows.PURGE_RXCLEAR|windows.PURGE_RXABORT,
	); err != nil {
		_ = cfg.s.close()
		return fmt.Errorf("PurgeComm failed: %w", err)
	}

	return nil
}

// ClearCommError + COMSTAT.cbOutQue / cbInQue
func (p *port) getBytesToWrite() (int, error) {
	if !p.isOpen() {
		return 0, errors.New("serial port is not open")
	}
	var flags uint32
	var st windows.ComStat
	if err := windows.ClearCommError(p.h, &flags, &st); err != nil {
		_ = p.close()
		return 0, fmt.Errorf("getBytesToWrite failed: %w", err)
	}
	return int(st.CBOutQue), nil
}

func (p *port) getBytesToRead() (int, error) {
	if !p.isOpen() {
		return 0, errors.New("serial port is not open")
	}
	var flags uint32
	var st windows.ComStat
	if err := windows.ClearCommError(p.h, &flags, &st); err != nil {
		if err != windows.ERROR_INVALID_HANDLE {
			_ = p.close()
			return 0, fmt.Errorf("getBytesToRead failed: %w", err)
		}
		return 0, nil
	}
	return int(st.CBInQue), nil
}

func (p *port) read() ([]byte, error) {
	if p.closing == 0 {
		return nil, nil
	}
	if !p.isOpen() {
		return nil, errors.New("serial port is not open")
	}

	count, err := p.getBytesToRead()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		count = 1
	}

	buf := make([]byte, count)
	var n uint32
	_ = windows.ResetEvent(p.ovRead.HEvent)
	err = windows.ReadFile(p.h, buf, &n, &p.ovRead)
	if err == nil {
		return buf[:n], nil
	}
	if !errors.Is(err, windows.ERROR_IO_PENDING) {
		r, err := windows.WaitForSingleObject(p.closing, 1)
		if p.closing == 0 || r == windows.WAIT_OBJECT_0 && err == nil {
			//If app is closing.
			return nil, nil
		}
		return nil, fmt.Errorf("read failed: %w", err)
	}
	handles := []windows.Handle{p.closing, p.ovRead.HEvent}
	idx, werr := windows.WaitForMultipleObjects(handles, false, windows.INFINITE)
	if werr != nil {
		r, err := windows.WaitForSingleObject(p.closing, 1)
		if p.closing == 0 || r == windows.WAIT_OBJECT_0 && err == nil {
			//If app is closing.
			return nil, nil
		}
		return nil, fmt.Errorf("read wait failed: %w", werr)
	}
	if idx == windows.WAIT_OBJECT_0 {
		return nil, nil // closing
	}
	if gerr := windows.GetOverlappedResult(p.h, &p.ovRead, &n, true); gerr != nil {
		if errors.Is(gerr, windows.ERROR_OPERATION_ABORTED) {
			return nil, nil
		}
		r, err := windows.WaitForSingleObject(p.closing, 0)
		if r == windows.WAIT_OBJECT_0 && err == nil {
			//If app is closing.
			return nil, nil
		}
		return nil, fmt.Errorf("read failed: %w", gerr)
	}
	count, err = p.getBytesToRead()
	if err != nil {
		return nil, err
	}
	if count != 0 {
		ret, err := p.read()
		if err != nil {
			return nil, err
		}
		return append(buf[:n], ret...), nil
	}
	return buf[:n], nil
}

func (p *port) write(data []byte) (int, error) {
	if !p.isOpen() {
		return 0, errors.New("serial port is not open")
	}
	if len(data) == 0 {
		return 0, nil
	}

	var n uint32

	_ = windows.ResetEvent(p.ovWrite.HEvent)

	err := windows.WriteFile(p.h, data, &n, &p.ovWrite)
	if err == nil {
		return len(data), nil
	}

	if errors.Is(err, windows.ERROR_INVALID_USER_BUFFER) ||
		errors.Is(err, windows.ERROR_NOT_ENOUGH_MEMORY) ||
		errors.Is(err, windows.ERROR_OPERATION_ABORTED) {
		return 0, nil
	}

	if errors.Is(err, windows.ERROR_IO_PENDING) {
		timeout := uint32((1 * time.Second) / time.Millisecond)
		handles := []windows.Handle{p.closing, p.ovWrite.HEvent}
		idx, werr := windows.WaitForMultipleObjects(handles, false, timeout)
		if werr != nil {
			return 0, fmt.Errorf("write wait failed: %w", werr)
		}
		if idx == windows.WAIT_OBJECT_0 {
			return 0, nil // closing
		}
		if gerr := windows.GetOverlappedResult(p.h, &p.ovWrite, &n, true); gerr != nil {
			if errors.Is(gerr, windows.ERROR_OPERATION_ABORTED) {
				return 0, nil
			}
			return 0, fmt.Errorf("write failed: %w", gerr)
		}
		return len(data), nil
	}

	return 0, fmt.Errorf("write failed: %w", err)
}

func (p *port) close() error {
	if p == nil {
		return nil
	}
	if p.closing != 0 {
		_ = windows.SetEvent(p.closing)
	}
	if p.h != 0 && p.h != windows.InvalidHandle {
		_ = windows.CancelIoEx(p.h, nil)
	}

	if p.ovRead.HEvent != 0 {
		_ = windows.CloseHandle(p.ovRead.HEvent)
		p.ovRead.HEvent = 0
	}
	if p.ovWrite.HEvent != 0 {
		_ = windows.CloseHandle(p.ovWrite.HEvent)
		p.ovWrite.HEvent = 0
	}
	if p.h != 0 {
		_ = windows.CloseHandle(p.h)
		p.h = 0
	}
	if p.closing != 0 {
		_ = windows.CloseHandle(p.closing)
		p.closing = 0
	}
	return nil
}
