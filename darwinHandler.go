//go:build darwin

package gxserial

// --------------------------------------------------------------------------
//
//	Gurux Ltd
//
// Filename:        $HeadURL$
//
// Version:         $Revision$,
//
//	$Date$
//	$Author$
//
// # Copyright (c) Gurux Ltd
//
// ---------------------------------------------------------------------------
//
//	DESCRIPTION
//
// This file is a part of Gurux Device Framework.
//
// Gurux Device Framework is Open Source software; you can redistribute it
// and/or modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation; version 2 of the License.
// Gurux Device Framework is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU General Public License for more details.
//
// More information of Gurux products: https://www.gurux.org
//
// This code is licensed under the GNU General Public License v2.
// Full text may be retrieved at http://www.gnu.org/licenses/gpl-2.0.txt
// ---------------------------------------------------------------------------

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/Gurux/gxcommon-go"
	"golang.org/x/sys/unix"
)

type port struct {
	f  *os.File
	fd int
	r  *os.File
	w  *os.File
}

// toUnitBaudrate maps a baud rate to the corresponding constant in the mac package.
var toUnitBaudrate = map[int]uint32{
	0:      unix.B0,
	50:     unix.B50,
	75:     unix.B75,
	110:    unix.B110,
	134:    unix.B134,
	150:    unix.B150,
	200:    unix.B200,
	300:    unix.B300,
	600:    unix.B600,
	1200:   unix.B1200,
	1800:   unix.B1800,
	2400:   unix.B2400,
	4800:   unix.B4800,
	9600:   unix.B9600,
	19200:  unix.B19200,
	38400:  unix.B38400,
	57600:  unix.B57600,
	115200: unix.B115200,
}

// getPortNames returns a list of available serial port device paths on macOS.
func getPortNames() ([]string, error) {
	patterns := []string{
		"/dev/tty.*",
		"/dev/cu.*",
	}

	var devices []string
	seen := make(map[string]struct{})
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, device := range matches {
			if _, ok := seen[device]; !ok {
				seen[device] = struct{}{}
				devices = append(devices, device)
			}
		}
	}
	return devices, nil
}

func openPort(cfg *GXSerial) error {
	fd, err := unix.Open(cfg.Port, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0666)
	if err != nil {
		return err
	}

	f := os.NewFile(uintptr(fd), cfg.Port)
	cfg.s = port{f: f, fd: fd}

	// (iflag, oflag, cflag, lflag, ispeed, ospeed, cc) = tcgetattr
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		cfg.s.close()
		return err
	}
	t.Cflag |= unix.CLOCAL | unix.CREAD
	t.Lflag &^= unix.ICANON | unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ECHONL | unix.ISIG | unix.IEXTEN
	t.Oflag &^= unix.OPOST | unix.ONLCR | unix.OCRNL
	t.Iflag &^= unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IGNBRK
	// Baud rate:
	speed := toUnitBaudrate[int(cfg.baudRate)]
	t.Ispeed = uint64(speed)
	t.Ospeed = uint64(speed)
	// Databits:
	t.Cflag &^= unix.CSIZE
	switch cfg.dataBits {
	case 5:
		t.Cflag |= unix.CS5
	case 6:
		t.Cflag |= unix.CS6
	case 7:
		t.Cflag |= unix.CS7
	case 8:
		t.Cflag |= unix.CS8
	default:
		cfg.s.close()
		return errors.New("invalid databits (must be 5..8)")
	}

	// Stop bits
	switch cfg.stopBits {
	case 1:
		t.Cflag &^= unix.CSTOPB
	case 2:
		t.Cflag |= unix.CSTOPB
	default:
		cfg.s.close()
		return errors.New("invalid stopbits (must be 1 or 2)")
	}

	// setup parity
	t.Iflag &^= unix.INPCK | unix.ISTRIP

	const CMSPAR = 0x40000000
	hasCMSPAR := false
	t.Cflag &^= unix.PARENB | unix.PARODD
	if hasCMSPAR {
		t.Cflag &^= CMSPAR
	}

	switch cfg.parity {
	case gxcommon.ParityNone:
		// No parity: parity bit off, no parity checking
	case gxcommon.ParityEven:
		t.Cflag |= unix.PARENB
		t.Cflag &^= unix.PARODD
		if hasCMSPAR {
			t.Cflag &^= CMSPAR
		}
	case gxcommon.ParityOdd:
		t.Cflag |= unix.PARENB | unix.PARODD
		if hasCMSPAR {
			t.Cflag &^= CMSPAR
		}
	case gxcommon.ParityMark:
		if !hasCMSPAR {
			cfg.s.close()
			return errors.New("mark parity requested but CMSPAR not supported")
		}
		t.Cflag |= unix.PARENB | CMSPAR | unix.PARODD
	case gxcommon.ParitySpace:
		if !hasCMSPAR {
			cfg.s.close()
			return errors.New("space parity requested but CMSPAR not supported")
		}
		t.Cflag |= unix.PARENB | CMSPAR
		t.Cflag &^= unix.PARODD
	default:
		cfg.s.close()
		return errors.New("invalid parity")
	}

	t.Iflag &^= unix.IXON | unix.IXOFF
	t.Cflag &^= unix.CRTSCTS
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, t); err != nil {
		cfg.s.close()
		return err
	}
	if err := ioctlSetIntPointer(fd, unix.TIOCFLUSH, unix.TCIOFLUSH); err != nil {
		cfg.s.close()
		return err
	}
	cfg.s.r, cfg.s.w, err = os.Pipe()
	if err != nil {
		cfg.s.close()
		return err
	}
	_ = unix.SetNonblock(int(cfg.s.r.Fd()), true)
	return nil
}

func ioctlSetIntPointer(fd int, req uint, value int) error {
	v := value
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v)))
	if errno != 0 {
		return errno
	}
	return nil
}

func (p *port) close() error {
	if p == nil {
		return nil
	}
	if p.r != nil {
		_ = p.r.Close()
		p.r = nil
	}
	if p.w != nil {
		_ = p.w.Close()
		p.w = nil
	}
	if p.f != nil {
		f := p.f
		p.f = nil
		p.fd = 0
		return f.Close()
	}
	return nil
}

func (p *port) isOpen() bool {
	return p.f != nil
}

func (p *port) ensureOpen() error {
	if p == nil || p.f == nil {
		return errors.New("serial port not open")
	}
	return nil
}

func (p *port) getTermios() (*unix.Termios, error) {
	if err := p.ensureOpen(); err != nil {
		return nil, err
	}
	t, err := unix.IoctlGetTermios(p.fd, unix.TIOCGETA)
	if err != nil {
		return nil, fmt.Errorf("tcgetattr failed: %w", err)
	}
	return t, nil
}

func (p *port) setTermios(value *unix.Termios) error {
	if err := p.ensureOpen(); err != nil {
		return err
	}
	if err := unix.IoctlSetTermios(p.fd, unix.TIOCSETA, value); err != nil {
		return fmt.Errorf("tcsetattr failed: %w", err)
	}
	return nil
}

func (p *port) setBaudRate(value gxcommon.BaudRate) error {
	t, err := p.getTermios()
	if err != nil {
		return fmt.Errorf("setBaudRate failed. %w", err)
	}
	u := toUnitBaudrate[int(value)]
	if u == 0 {
		return fmt.Errorf("setBaudRate failed. unsupported baud: %d", value)
	}
	t.Ispeed = uint64(u)
	t.Ospeed = uint64(u)
	return p.setTermios(t)
}

func (p *port) setDataBits(value int) error {
	t, err := p.getTermios()
	if err != nil {
		return fmt.Errorf("setDataBits failed. %w", err)
	}
	t.Cflag &^= unix.CSIZE
	switch value {
	case 5:
		t.Cflag |= unix.CS5
	case 6:
		t.Cflag |= unix.CS6
	case 7:
		t.Cflag |= unix.CS7
	case 8:
		t.Cflag |= unix.CS8
	default:
		return fmt.Errorf("setDataBits failed. invalid databits: %d", value)
	}
	return p.setTermios(t)
}

func (p *port) setParity(value gxcommon.Parity) error {
	t, err := p.getTermios()
	if err != nil {
		return fmt.Errorf("setParity failed. %w", err)
	}
	t.Cflag &^= unix.PARENB | unix.PARODD
	switch value {
	case gxcommon.ParityNone:
		// nothing
	case gxcommon.ParityEven:
		t.Cflag |= unix.PARENB
	case gxcommon.ParityOdd:
		t.Cflag |= unix.PARENB | unix.PARODD
	case gxcommon.ParityMark, gxcommon.ParitySpace:
		return fmt.Errorf("mark/space parity not supported on this system")
	}
	return p.setTermios(t)
}

func (p *port) getStopBits() (int, error) {
	t, err := p.getTermios()
	if err != nil {
		return 0, fmt.Errorf("getStopBits failed. %w", err)
	}
	if (t.Cflag & unix.CSTOPB) != 0 {
		return 2, nil
	}
	return 1, nil
}

func (p *port) setStopBits(value gxcommon.StopBits) error {
	t, err := p.getTermios()
	if err != nil {
		return fmt.Errorf("setStopBits failed. %w", err)
	}
	t.Cflag &^= unix.CSTOPB
	if value == gxcommon.StopBitsTwo {
		t.Cflag |= unix.CSTOPB
	} else if value != gxcommon.StopBitsOne {
		return fmt.Errorf("setStopBits failed. invalid value: %d (use StopBitsOne or StopBitsTwo)", value)
	}
	return p.setTermios(t)
}

func (p *port) getBytesToRead() (int, error) {
	if err := p.ensureOpen(); err != nil {
		return 0, err
	}
	pfds := []unix.PollFd{{Fd: int32(p.fd), Events: unix.POLLIN}}
	_, err := unix.Poll(pfds, 0)
	if err != nil {
		return 0, fmt.Errorf("getBytesToRead failed: %w", err)
	}
	if (pfds[0].Revents & unix.POLLIN) != 0 {
		return 1, nil
	}
	return 0, nil
}

func (p *port) getBytesToWrite() (int, error) {
	if err := p.ensureOpen(); err != nil {
		return 0, err
	}
	n, err := unix.IoctlGetInt(p.fd, unix.TIOCOUTQ)
	if err != nil {
		return 0, fmt.Errorf("getBytesToWrite failed: %w", err)
	}
	return n, nil
}

func (p *port) getRtsEnable() (bool, error) {
	if err := p.ensureOpen(); err != nil {
		return false, err
	}
	status, err := unix.IoctlGetInt(p.fd, unix.TIOCMGET)
	if err != nil {
		return false, fmt.Errorf("getRtsEnable failed: %w", err)
	}
	return (status & unix.TIOCM_RTS) != 0, nil
}

func (p *port) setRtsEnable(on bool) error {
	return p.setModemBit(unix.TIOCM_RTS, on)
}

func (p *port) getDtrEnable() (bool, error) {
	if err := p.ensureOpen(); err != nil {
		return false, err
	}
	status, err := unix.IoctlGetInt(p.fd, unix.TIOCMGET)
	if err != nil {
		return false, fmt.Errorf("getDtrEnable failed: %w", err)
	}
	return (status & unix.TIOCM_DTR) != 0, nil
}

func (p *port) setDtrEnable(on bool) error {
	return p.setModemBit(unix.TIOCM_DTR, on)
}

func (p *port) setModemBit(bit int, on bool) error {
	if err := p.ensureOpen(); err != nil {
		return err
	}
	v := bit
	req := unix.TIOCMBIC
	if on {
		req = unix.TIOCMBIS
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(p.fd), uintptr(req), uintptr(unsafe.Pointer(&v)))
	if errno != 0 {
		return fmt.Errorf("set modem bit failed: %v", errno)
	}
	return nil
}

func (p *port) read() ([]byte, error) {
	if err := p.ensureOpen(); err != nil {
		return nil, err
	}
	if p.r == nil {
		return nil, errors.New("read not initialized: closedR is nil")
	}

	pfds := []unix.PollFd{
		{Fd: int32(p.fd), Events: unix.POLLIN},
		{Fd: int32(p.r.Fd()), Events: unix.POLLIN},
	}
	//For some reasons close might hang sometimes if infinity value is used.
	_, err := unix.Poll(pfds, 100)
	if err != nil {
		return nil, err
	}
	if (pfds[1].Revents & unix.POLLIN) != 0 {
		return nil, nil
	}

	cnt, _ := p.getBytesToRead()
	if cnt <= 0 {
		cnt = 1
	}
	buf := make([]byte, cnt)
	n, err := p.f.Read(buf)
	if err != nil {
		return nil, err
	}
	cnt, _ = p.getBytesToRead()
	if cnt != 0 {
		ret, err := p.read()
		if err != nil {
			return nil, err
		}
		return append(buf[:n], ret...), nil
	}
	return buf[:n], nil
}

func (p *port) write(data []byte) (int, error) {
	if err := p.ensureOpen(); err != nil {
		return 0, err
	}
	return p.f.Write(data)
}
