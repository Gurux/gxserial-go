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

// GXSerial holds connection configuration and tracing settings for a network media.
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

// NewGXSerial creates a GXSerial configured with the given serial port.
func NewGXSerial(port string,
	baudRate gxcommon.BaudRate,
	dataBits int,
	parity gxcommon.Parity,
	stopBits gxcommon.StopBits) *GXSerial {
	g := &GXSerial{Port: port, baudRate: baudRate, dataBits: dataBits, stopBits: stopBits, parity: parity, stop: make(chan struct{})}
	g.Localize(language.AmericanEnglish)
	g.received = *newGXSynchronousMediaBase()
	return g
}

// GetPortNames retrurns list of available serial ports.
func GetPortNames() ([]string, error) {
	return getPortNames()
}

// BaudRate returns the used baud rate.
func (g *GXSerial) BaudRate() gxcommon.BaudRate {
	return g.baudRate
}

// SetBaudRate sets the used baud rate.
func (g *GXSerial) SetBaudRate(value gxcommon.BaudRate) error {
	g.baudRate = value
	if g.s.isOpen() {
		return g.s.setBaudRate(value)
	}
	return nil
}

// DataBits returns the amount of the data bits.
func (g *GXSerial) DataBits() int {
	return g.dataBits
}

// SetDataBits  sets the amount of the data bits.
func (g *GXSerial) SetDataBits(value int) error {
	g.dataBits = value
	if g.s.isOpen() {
		return g.s.setDataBits(value)
	}
	return nil
}

// StopBits returns used stop bits.
func (g *GXSerial) StopBits() gxcommon.StopBits {
	return g.stopBits
}

// SetStopBits sets the used stop bits.
func (g *GXSerial) SetStopBits(value gxcommon.StopBits) error {
	g.stopBits = value
	if g.s.isOpen() {
		return g.s.setStopBits(value)
	}
	return nil
}

// Parity returns used parity.
func (g *GXSerial) Parity() gxcommon.Parity {
	return g.parity
}

// SetParity sets the used parity.
func (g *GXSerial) SetParity(value gxcommon.Parity) error {
	g.parity = value
	if g.s.isOpen() {
		return g.s.setParity(value)
	}
	return nil
}

// GetBytesToRead returns the number of bytes currently available to read.
func (g *GXSerial) GetBytesToRead() (int, error) {
	if g.s.isOpen() {
		return g.s.getBytesToRead()
	}
	return 0, nil
}

// GetBytesToWrite returns the number of bytes currently available to write.
func (g *GXSerial) GetBytesToWrite() (int, error) {
	if g.s.isOpen() {
		return g.s.getBytesToWrite()
	}
	return 0, nil
}

// String implements IGXMedia
func (g *GXSerial) String() string {
	return fmt.Sprintf("%s %s %d %s %s", g.Port, g.baudRate, g.dataBits, g.stopBits, g.parity)
}

// GetName implements IGXMedia
func (g *GXSerial) GetName() string {
	return fmt.Sprint(g.Port)
}

// IsOpen implements IGXMedia
func (g *GXSerial) IsOpen() bool {
	return g.s.isOpen()
}

// Copy implements IGXMedia
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

// GetMediaType implements IGXMedia
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

// GetSettings implements IGXMedia
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

// SetSettings implements IGXMedia
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

// GetSynchronous implements IGXMedia
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

// IsSynchronous implements IGXMedia
func (g *GXSerial) IsSynchronous() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.synchronous
}

// ResetSynchronousBuffer implements IGXMedia
func (g *GXSerial) ResetSynchronousBuffer() {
}

// GetBytesSent implements IGXMedia
func (g *GXSerial) GetBytesSent() uint64 {
	return g.bytesSent
}

// GetBytesReceived implements IGXMedia
func (g *GXSerial) GetBytesReceived() uint64 {
	return g.bytesReceived
}

// ResetByteCounters implements IGXMedia
func (g *GXSerial) ResetByteCounters() {
	g.bytesSent = 0
	g.bytesReceived = 0
}

// Validate implements IGXMedia
func (g *GXSerial) Validate() error {
	if g.Port == "" {
		return errors.New(g.p.Sprintf("msg.no_serial_port_selected"))
	}
	return nil
}

// SetEop implements IGXMedia
func (g *GXSerial) SetEop(eop any) {
	g.eop = eop
}

// GetEop implements IGXMedia
func (g *GXSerial) GetEop() any {
	return g.eop
}

// GetTrace implements IGXMedia
func (g *GXSerial) GetTrace() gxcommon.TraceLevel {
	return g.traceLevel
}

// SetTrace implements IGXMedia
func (g *GXSerial) SetTrace(traceLevel gxcommon.TraceLevel) error {
	g.traceLevel = traceLevel
	return nil
}

// SetOnReceived implements IGXMedia
func (g *GXSerial) SetOnReceived(value gxcommon.ReceivedEventHandler) {
	g.mu.Lock()
	g.onReceive = value
	g.mu.Unlock()
}

// SetOnError implements IGXMedia
func (g *GXSerial) SetOnError(value gxcommon.ErrorEventHandler) {
	g.mu.Lock()
	g.onErr = value
	g.mu.Unlock()
}

// SetOnMediaStateChange implements IGXMedia
func (g *GXSerial) SetOnMediaStateChange(value gxcommon.MediaStateHandler) {
	g.mu.Lock()
	g.onState = value
	g.mu.Unlock()
}

// SetOnTrace implements IGXMedia
func (g *GXSerial) SetOnTrace(value gxcommon.TraceEventHandler) {
	g.mu.Lock()
	g.onTrace = value
	g.mu.Unlock()
}

// Open implements IGXMedia
func (g *GXSerial) Open() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.s.isOpen() {
		return nil
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

// Send implements IGXMedia
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

// Receive implements IGXMedia
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
		if err != nil {
			// timeout
			if (g.stop) != nil {
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

// Close implements IGXMedia
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
	message.SetString(language.AmericanEnglish, "msg.closing_connection", "Closing connection to %s")
	message.SetString(language.AmericanEnglish, "msg.connection_closed", "Connection closed to %s")
	message.SetString(language.AmericanEnglish, "msg.connection_failed", "Connection failed: %v")
	message.SetString(language.AmericanEnglish, "msg.count_or_eop", "Either Count or EOP must be set")
	message.SetString(language.AmericanEnglish, "msg.connected_to", "Connected to %s:")
	message.SetString(language.AmericanEnglish, "msg.connect_failed", "connect to %s: failed: %v")
	message.SetString(language.AmericanEnglish, "msg.connecting_to", "%s connecting to %s: timeout %d ms")
	message.SetString(language.AmericanEnglish, "msg.no_serial_port_selected", "No serial port selected. Please select a serial port.")

	// --- German (de) ---
	message.SetString(language.German, "msg.closing_connection", "Verbindung zu %s: wird geschlossen")
	message.SetString(language.German, "msg.connection_closed", "Verbindung zu %s: wurde geschlossen")
	message.SetString(language.German, "msg.connection_failed", "Verbindung fehlgeschlagen: %v")
	message.SetString(language.German, "msg.count_or_eop", "Entweder Count oder EOP muss gesetzt sein")
	message.SetString(language.German, "msg.connected_to", "Verbunden mit %s:")
	message.SetString(language.German, "msg.connect_failed", "Verbindung zu %s: fehlgeschlagen: %v")
	message.SetString(language.German, "msg.connecting_to", "%s verbindet sich mit %s: timeout %d ms")
	message.SetString(language.AmericanEnglish, "msg.no_serial_port_selected", "Kein serieller Port ausgewählt. Bitte wählen Sie einen seriellen Port aus.")

	// --- Finnish (fi) ---
	message.SetString(language.Finnish, "msg.closing_connection", "Suljetaan yhteys kohteeseen %s:")
	message.SetString(language.Finnish, "msg.connection_closed", "Yhteys suljettu kohteeseen %s:")
	message.SetString(language.Finnish, "msg.connection_failed", "Yhteyden muodostus epäonnistui: %v")
	message.SetString(language.Finnish, "msg.count_or_eop", "Joko Count tai EOP on asetettava")
	message.SetString(language.Finnish, "msg.connected_to", "Yhdistetty kohteeseen %s:")
	message.SetString(language.Finnish, "msg.connect_failed", "Yhteyden muodostus kohteeseen %s: epäonnistui: %v")
	message.SetString(language.Finnish, "msg.connecting_to", "%s yhdistetään kohteeseen %s: timeout %d ms")
	message.SetString(language.AmericanEnglish, "msg.no_serial_port_selected", "Sarjaporttia ei ole valittu. Valitse sarjaportti.")

	// --- Swedish (sv) ---
	message.SetString(language.Swedish, "msg.closing_connection", "Stänger anslutning till %s:")
	message.SetString(language.Swedish, "msg.connection_closed", "Anslutning stängd till %s:")
	message.SetString(language.Swedish, "msg.connection_failed", "Anslutningen misslyckades: %v")
	message.SetString(language.Swedish, "msg.count_or_eop", "Antingen Count eller EOP måste anges")
	message.SetString(language.Swedish, "msg.connected_to", "Ansluten till %s:")
	message.SetString(language.Swedish, "msg.connect_failed", "Anslutning till %s: misslyckades: %v")
	message.SetString(language.Swedish, "msg.connecting_to", "%s ansluter till %s: timeout %d ms")
	message.SetString(language.AmericanEnglish, "msg.no_serial_port_selected", "Ingen seriell port vald. Välj en seriell port.")

	// --- Spanish (es) ---
	message.SetString(language.Spanish, "msg.closing_connection", "Cerrando conexión con %s:")
	message.SetString(language.Spanish, "msg.connection_closed", "Conexión cerrada con %s:")
	message.SetString(language.Spanish, "msg.connection_failed", "Error de conexión: %v")
	message.SetString(language.Spanish, "msg.count_or_eop", "Se debe establecer Count o EOP")
	message.SetString(language.Spanish, "msg.connected_to", "Conectado a %s:")
	message.SetString(language.Spanish, "msg.connect_failed", "Error al conectar con %s:: %v")
	message.SetString(language.Spanish, "msg.connecting_to", "%s conectando a %s: timeout %d ms")
	message.SetString(language.AmericanEnglish, "msg.no_serial_port_selected", "No se ha seleccionado ningún puerto serie. Seleccione un puerto serie.")

	// --- Estonian (et) ---
	message.SetString(language.Estonian, "msg.closing_connection", "Suletakse ühendus sihtkohta %s:")
	message.SetString(language.Estonian, "msg.connection_closed", "Ühendus suleti sihtkohta %s:")
	message.SetString(language.Estonian, "msg.connection_failed", "Ühendus ebaõnnestus: %v")
	message.SetString(language.Estonian, "msg.count_or_eop", "Count või EOP peab olema määratud")
	message.SetString(language.Estonian, "msg.connected_to", "Ühendatud sihtkohta %s:")
	message.SetString(language.Estonian, "msg.connect_failed", "Ühendamine sihtkohta %s: ebaõnnestus: %v")
	message.SetString(language.Estonian, "msg.connecting_to", "%s ühendatakse sihtkohta %s: timeout %d ms")
	message.SetString(language.AmericanEnglish, "msg.no_serial_port_selected", "Ühtegi jadaporti pole valitud. Palun valige jadaport.")
}

// Localize messages for the specified language.
// No errors is returned if language is not supported.
func (g *GXSerial) Localize(language language.Tag) {
	g.p = message.NewPrinter(language)
}
