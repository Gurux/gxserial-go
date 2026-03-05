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
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Gurux/gxcommon-go"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// GXSerial implements serial-port-based media that satisfies gxcommon.IGXMedia.
// It stores serial configuration, I/O counters, event callbacks, and trace settings.
type GXSerial struct {
	Port     string
	baudRate gxcommon.BaudRate
	dataBits int
	stopBits gxcommon.StopBits
	parity   gxcommon.Parity
	eop      any
	// The trace level specifies which types of trace messages are emitted.
	traceLevel gxcommon.TraceLevel
	// OnReceived: Media component notifies asynchronously received data through this method.
	mu sync.RWMutex
	wg sync.WaitGroup

	stop        chan struct{}
	synchronous bool

	bytesSent     uint64
	bytesReceived uint64

	//Called when the Media state is changed.
	onState gxcommon.MediaStateHandler

	//Called when the new data is received.
	onReceive gxcommon.ReceivedEventHandler

	//Called when the Media is sending or receiving data.
	onTrace gxcommon.TraceEventHandler

	//Called when the Media is sending or receiving data.
	onErr gxcommon.ErrorEventHandler

	//Sync settings.
	receivedSize int
	received     synchronousMediaBase

	s port
	// Printer for localized messages.
	p *message.Printer
}

// NewGXSerial creates a GXSerial with the given serial settings.
// The returned instance uses American English localization by default for error messages.
//
// Example:
//
//	media := NewGXSerial("COM1", gxcommon.BaudRate(9600), 8, gxcommon.ParityNone, gxcommon.StopBitsOne)
//	if err := media.Open(); err != nil {
//		// handle open error
//	}
//	defer media.Close()
func NewGXSerial(port string,
	baudRate gxcommon.BaudRate,
	dataBits int,
	parity gxcommon.Parity,
	stopBits gxcommon.StopBits) *GXSerial {
	g := &GXSerial{Port: port, baudRate: baudRate, dataBits: dataBits, stopBits: stopBits, parity: parity, stop: make(chan struct{})}
	g.received = *newGXSynchronousMediaBase()
	g.p = message.NewPrinter(gxcommon.Language())
	return g
}

// GetPortNames returns the list of available serial ports for the
// current operating system.  The returned slice may be empty if no ports are found or an error occurs.
// Port names are platform‑specific (e.g., "COM1" on Windows, "/dev/ttyUSB0" on Linux).
func GetPortNames() ([]string, error) {
	return getPortNames()
}

// BaudRate returns the configured baud rate.
func (g *GXSerial) BaudRate() gxcommon.BaudRate {
	return g.baudRate
}

// SetBaudRate updates the configured baud rate.
// If the serial port is already open, the new value is applied immediately.
func (g *GXSerial) SetBaudRate(value gxcommon.BaudRate) error {
	g.baudRate = value
	if g.s.isOpen() {
		return g.s.setBaudRate(value)
	}
	return nil
}

// DataBits returns the configured number of data bits.
func (g *GXSerial) DataBits() int {
	return g.dataBits
}

// SetDataBits updates the configured number of data bits.
// If the serial port is already open, the new value is applied immediately.
func (g *GXSerial) SetDataBits(value int) error {
	g.dataBits = value
	if g.s.isOpen() {
		return g.s.setDataBits(value)
	}
	return nil
}

// StopBits returns the configured stop bits.
func (g *GXSerial) StopBits() gxcommon.StopBits {
	return g.stopBits
}

// SetStopBits updates the configured stop bits.
// If the serial port is already open, the new value is applied immediately.
func (g *GXSerial) SetStopBits(value gxcommon.StopBits) error {
	g.stopBits = value
	if g.s.isOpen() {
		return g.s.setStopBits(value)
	}
	return nil
}

// Parity returns the configured parity mode.
func (g *GXSerial) Parity() gxcommon.Parity {
	return g.parity
}

// SetParity updates the configured parity mode.
// If the serial port is already open, the new value is applied immediately.
func (g *GXSerial) SetParity(value gxcommon.Parity) error {
	g.parity = value
	if g.s.isOpen() {
		return g.s.setParity(value)
	}
	return nil
}

// GetBytesToRead returns bytes currently available in the read buffer.
// If the port is closed, zero is returned.
func (g *GXSerial) GetBytesToRead() (int, error) {
	if g.s.isOpen() {
		return g.s.getBytesToRead()
	}
	return 0, nil
}

// GetBytesToWrite returns bytes currently queued in the write buffer.
// If the port is closed, zero is returned.
func (g *GXSerial) GetBytesToWrite() (int, error) {
	if g.s.isOpen() {
		return g.s.getBytesToWrite()
	}
	return 0, nil
}

// String returns the serial settings as a human-readable string.
func (g *GXSerial) String() string {
	return fmt.Sprintf("%s %s %d %s %s", g.Port, g.baudRate, g.dataBits, g.stopBits, g.parity)
}

// GetName returns the media name, which is the configured serial port.
func (g *GXSerial) GetName() string {
	return fmt.Sprint(g.Port)
}

// IsOpen reports whether the serial port is currently open.
func (g *GXSerial) IsOpen() bool {
	return g.s.isOpen()
}

// Copy copies serial configuration and trace settings into target.
// Target must be *GXSerial.
func (g *GXSerial) Copy(target gxcommon.IGXMedia) error {
	switch dst := target.(type) {
	case *GXSerial:
		dst.Port = g.Port
		dst.baudRate = g.baudRate
		dst.dataBits = g.dataBits
		dst.stopBits = g.stopBits
		dst.parity = g.parity
		dst.traceLevel = g.traceLevel
		dst.eop = g.eop
	default:
		return fmt.Errorf("copy: target is %T; want *GXSerial", target)
	}
	return nil
}

// GetMediaType returns the media type name.
func (g *GXSerial) GetMediaType() string {
	return "Serial"
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(s)); err != nil {
		return s
	}
	return buf.String()
}

// GetSettings serializes current serial settings into an XML fragment.
func (g *GXSerial) GetSettings() string {
	var b strings.Builder
	if g.Port != "" {
		fmt.Fprintf(&b, "<Port>%s</Port>\n", xmlEscape(g.Port))
	}
	if g.baudRate != 0 {
		fmt.Fprintf(&b, "<Bps>%d</Bps>\n", g.baudRate)
	}
	if g.dataBits != 0 {
		fmt.Fprintf(&b, "<ByteSize>%d</ByteSize>\n", g.dataBits)
	}
	if g.stopBits != 0 {
		fmt.Fprintf(&b, "<StopBits>%d</StopBits>\n", g.stopBits)
	}
	if g.parity != 0 {
		fmt.Fprintf(&b, "<Parity>%d</Parity>\n", g.parity)
	}
	return b.String()
}

// SetSettings loads serial settings from an XML fragment.
// Unknown tags are ignored.
func (g *GXSerial) SetSettings(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	dec := xml.NewDecoder(strings.NewReader("<root>" + value + "</root>"))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		switch se.Name.Local {
		case "Port":
			var v string
			if err := dec.DecodeElement(&v, &se); err != nil {
				return err
			}
			g.Port = v
		case "Bps":
			var v string
			if err := dec.DecodeElement(&v, &se); err != nil {
				return err
			}
			g.baudRate, err = gxcommon.BaudRateParse(v)
			if err != nil {
				return err
			}
		case "ByteSize":
			var v string
			if err := dec.DecodeElement(&v, &se); err != nil {
				return err
			}
			g.dataBits, err = strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid ByteSize value: %v", err)
			}
		case "StopBits":
			var v string
			if err := dec.DecodeElement(&v, &se); err != nil {
				return err
			}
			g.stopBits, err = gxcommon.StopBitsParse(v)
			if err != nil {
				return err
			}
		case "Parity":
			var v string
			if err := dec.DecodeElement(&v, &se); err != nil {
				return err
			}
			g.parity, err = gxcommon.ParityParse(v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// GetSynchronous enables synchronous receive mode and returns a restore function.
// Callers should defer the returned function to restore asynchronous mode.
func (g *GXSerial) GetSynchronous() func() {
	g.mu.Lock()
	g.synchronous = true
	g.mu.Unlock()
	return func() {
		g.mu.Lock()
		g.synchronous = false
		g.mu.Unlock()
	}
}

// IsSynchronous reports whether synchronous receive mode is enabled.
func (g *GXSerial) IsSynchronous() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.synchronous
}

// ResetSynchronousBuffer resets buffered synchronous receive state.
// This implementation is currently a no-op.
func (g *GXSerial) ResetSynchronousBuffer() {
}

// GetBytesSent returns the total number of bytes sent.
func (g *GXSerial) GetBytesSent() uint64 {
	return g.bytesSent
}

// GetBytesReceived returns the total number of bytes received.
func (g *GXSerial) GetBytesReceived() uint64 {
	return g.bytesReceived
}

// ResetByteCounters resets sent and received byte counters to zero.
func (g *GXSerial) ResetByteCounters() {
	g.bytesSent = 0
	g.bytesReceived = 0
}

// Validate checks that required serial settings are present.
func (g *GXSerial) Validate() error {
	if g.Port == "" {
		return errors.New(g.p.Sprintf("msg.no_serial_port_selected"))
	}
	return nil
}

// SetEop sets the end-of-packet (EOP) marker used by Receive.  The
// argument may be any value that gxcommon.ToBytes understands (byte, string,
// []byte, etc.).  When a non‑nil marker is configured, incoming bytes are
// buffered until the sequence is observed; nil disables framing and data is
// delivered raw.
func (g *GXSerial) SetEop(eop any) {
	g.eop = eop
}

// GetEop returns the currently configured end-of-packet marker, or nil if
// framing is disabled.  The returned value will be the same type provided to
// SetEop.
func (g *GXSerial) GetEop() any {
	return g.eop
}

// GetTrace returns the current trace level mask.  Higher levels produce
// more verbose output via the trace callback (see SetOnTrace).
func (g *GXSerial) GetTrace() gxcommon.TraceLevel {
	return g.traceLevel
}

// SetTrace configures the trace level mask.  Events with a severity lower
// than the given level will be ignored.  Trace events are delivered through the
// callback established with SetOnTrace.
func (g *GXSerial) SetTrace(traceLevel gxcommon.TraceLevel) error {
	g.traceLevel = traceLevel
	return nil
}

// SetOnReceived sets the callback for asynchronously received data.
func (g *GXSerial) SetOnReceived(value gxcommon.ReceivedEventHandler) {
	g.mu.Lock()
	g.onReceive = value
	g.mu.Unlock()
}

// SetOnError sets the callback for asynchronous media errors.
func (g *GXSerial) SetOnError(value gxcommon.ErrorEventHandler) {
	g.mu.Lock()
	g.onErr = value
	g.mu.Unlock()
}

// SetOnMediaStateChange sets the callback for media state transitions.
func (g *GXSerial) SetOnMediaStateChange(value gxcommon.MediaStateHandler) {
	g.mu.Lock()
	g.onState = value
	g.mu.Unlock()
}

// SetOnTrace sets the callback for trace events.
func (g *GXSerial) SetOnTrace(value gxcommon.TraceEventHandler) {
	g.mu.Lock()
	g.onTrace = value
	g.mu.Unlock()
}

// Open opens the serial port and starts the background reader goroutine.
// If the port is already open, Open returns nil.
func (g *GXSerial) Open() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.s.isOpen() {
		return nil
	}
	select {
	case <-g.stop:
		// Recreate stop channel when reopening after Close.
		g.stop = make(chan struct{})
	default:
	}
	g.statef(false, gxcommon.MediaStateOpening)
	g.trace(false, gxcommon.TraceTypesInfo, g.p.Sprintf("msg.connecting_to", g.Port))
	err := openPort(g)
	if err != nil {
		g.trace(false, gxcommon.TraceTypesError, g.p.Sprintf("msg.connect_failed", g.Port, err))
		g.errorf(false, err)
		return err
	}
	g.wg.Add(1)
	go g.reader()
	g.trace(false, gxcommon.TraceTypesInfo, g.p.Sprintf("msg.connected_to", g.Port))
	g.statef(false, gxcommon.MediaStateOpen)
	return nil
}

// Send writes data to the serial port and updates byte counters.
// Receiver is ignored for serial media.
func (g *GXSerial) Send(data any, receiver string) error {
	tmp, err := gxcommon.ToBytes(data, binary.BigEndian)
	if err != nil {
		return err
	}
	g.bytesSent += uint64(len(tmp))
	//Trace data.
	str, err := gxcommon.ToString(data)
	if err != nil {
		return err
	}
	g.tracef(true, gxcommon.TraceTypesSent, "TX: %s", str)
	_, ret := g.s.write(tmp)
	return ret
}

// Receive waits for data according to args and stores the converted value in args.Reply.
// It returns true when data was received and false on timeout.
func (g *GXSerial) Receive(args *gxcommon.ReceiveParameters) (bool, error) {
	if args.EOP == nil && args.Count == 0 && !args.AllData {
		return false, errors.New(g.p.Sprintf("msg.count_or_eop"))
	}
	terminator, err := gxcommon.ToBytes(args.EOP, binary.BigEndian)
	if err != nil {
		return false, err
	}

	var waitTime time.Duration
	if args.WaitTime <= 0 {
		waitTime = 0
	} else {
		waitTime = time.Duration(args.WaitTime) * time.Millisecond
	}
	index := g.received.Search(terminator, args.Count, waitTime)
	if index == -1 {
		return false, nil
	}

	if args.AllData {
		//Read all data.
		index = -1
	}
	args.Reply, err = gxcommon.BytesToAny2(g.received.Get(index), args.ReplyType, binary.ByteOrder(binary.BigEndian))
	if err != nil {
		return false, err
	}
	return true, nil
}

func (g *GXSerial) handleData(data []byte) {
	str, err := gxcommon.ToString(data)
	if err != nil {
		g.tracef(true, gxcommon.TraceTypesError, "RX failed: %v", err)
		g.errorf(true, err)
	} else {
		g.tracef(true, gxcommon.TraceTypesReceived, "RX: %s", str)
	}
	if g.synchronous {
		g.appendData(data)
	} else {
		g.receivef(true, data)
	}
}

func (g *GXSerial) reader() {
	defer g.wg.Done()
	for {
		ret, err := g.s.read()
		if !g.IsOpen() {
			return
		}
		if err != nil {
			select {
			case <-g.stop:
				return
			default:
				g.trace(false, gxcommon.TraceTypesError, g.p.Sprintf("msg.connection_failed", err))
				g.errorf(false, err)
			}
			return
		}
		if len(ret) != 0 {
			g.bytesReceived += uint64(len(ret))
			g.handleData(ret)
		}
		select {
		case <-g.stop:
			return
		default:
		}
	}
}

func (g *GXSerial) receivef(lock bool, data []byte) {
	var cb gxcommon.ReceivedEventHandler
	if lock {
		g.mu.RLock()
		cb = g.onReceive
		g.mu.RUnlock()
	} else {
		cb = g.onReceive
	}
	if cb != nil {
		cb(g, *gxcommon.NewReceiveEventArgs(data, g.Port))
	}
}

func (g *GXSerial) errorf(lock bool, err error) {
	var cb gxcommon.ErrorEventHandler
	if lock {
		g.mu.RLock()
		cb = g.onErr
		g.mu.RUnlock()
	} else {
		cb = g.onErr
	}
	if cb != nil {
		cb(g, err)
	}
}

func (g *GXSerial) tracef(lock bool, traceType gxcommon.TraceTypes, fmtStr string, a ...any) {
	var cb gxcommon.TraceEventHandler
	trace := false
	if lock {
		g.mu.RLock()
		trace = !(int(g.traceLevel) < int(traceType))
		cb = g.onTrace
		g.mu.RUnlock()
	} else {
		trace = !(int(g.traceLevel) < int(traceType))
		cb = g.onTrace
	}
	if cb != nil && trace {
		p := gxcommon.NewTraceEventArgs(traceType, fmt.Sprintf(fmtStr, a...), "")
		var m gxcommon.IGXMedia = g
		cb(m, *p)
	}
}

func (g *GXSerial) trace(lock bool, traceType gxcommon.TraceTypes, message string) {
	var cb gxcommon.TraceEventHandler
	trace := false
	if lock {
		g.mu.RLock()
		trace = !(int(g.traceLevel) < int(traceType))
		cb = g.onTrace
		g.mu.RUnlock()
	} else {
		trace = !(int(g.traceLevel) < int(traceType))
		cb = g.onTrace
	}
	if cb != nil && trace {
		p := gxcommon.NewTraceEventArgs(traceType, message, "")
		var m gxcommon.IGXMedia = g
		cb(m, *p)
	}
}

func (g *GXSerial) statef(lock bool, state gxcommon.MediaState) {
	var cb gxcommon.MediaStateHandler
	if lock {
		g.mu.RLock()
		cb = g.onState
		g.mu.RUnlock()
	} else {
		cb = g.onState
	}
	if cb != nil {
		cb(g, *gxcommon.NewMediaStateEventArgs(state))
	}
}

func (g *GXSerial) appendData(data []byte) {
	if len(data) == 0 {
		return
	}
	g.received.Append(data)
	g.mu.Lock()
	g.receivedSize += len(data)
	g.mu.Unlock()
}

// Close closes the serial port and waits for the reader goroutine to stop.
// It is safe to call Close multiple times.
func (g *GXSerial) Close() error {
	var err error
	g.mu.Lock()
	defer g.mu.Unlock()
	select {
	case <-g.stop:
		// already closed
	default:
		if g.s.isOpen() {
			g.trace(false, gxcommon.TraceTypesInfo, g.p.Sprintf("msg.closing_connection", g.Port))
			g.statef(false, gxcommon.MediaStateClosing)
		}
		_ = g.s.close()
		g.trace(false, gxcommon.TraceTypesInfo, g.p.Sprintf("msg.connection_closed", g.Port))
		g.statef(false, gxcommon.MediaStateClosed)
	}
	g.wg.Wait()
	return err
}

//nolint:errcheck
func init() {
	// --- English (default) ---
	message.SetString(language.AmericanEnglish, "msg.closing_connection", "Closing serial port '%s' connection")
	message.SetString(language.AmericanEnglish, "msg.connection_closed", "Serial port connection '%s' closed")
	message.SetString(language.AmericanEnglish, "msg.connection_failed", "Serial port connection failed: %v")
	message.SetString(language.AmericanEnglish, "msg.count_or_eop", "Either Count or EOP must be set")
	message.SetString(language.AmericanEnglish, "msg.connected_to", "Connected to serial port '%s'")
	message.SetString(language.AmericanEnglish, "msg.connect_failed", "Connect to serial port '%s' failed: %v")
	message.SetString(language.AmericanEnglish, "msg.connecting_to", "%s connecting to %s: timeout %d ms")
	message.SetString(language.AmericanEnglish, "msg.no_serial_port_selected", "No serial port selected. Please select a serial port.")
}
