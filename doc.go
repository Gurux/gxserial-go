// Package gxserial provides serial port based media for Gurux components.
// It implements the common IGXMedia-style contract: open/close a connection,
// send/receive data (optionally framed by an EOP marker), and emit events for
// received data, errors, tracing and state changes.
//
// Features
//
//   - Configurable serial settings (port, baud rate, data bits, parity, stop bits)
//   - Synchronous request/response and asynchronous receive callbacks
//   - Framing: optional EOP (End Of Packet) marker (byte, string or []byte).
//   - Timeouts: connection and I/O timeouts via time.Duration.
//   - Tracing: configurable trace level/mask for sent/received/error/info.
//   - Events: Received, Error, Trace and MediaState callbacks.
//   - Concurrency: safe for concurrent reads/writes; Close unblocks pending I/O.
//
// # Construction
//
// Use NewGXSerial to create a connection with used serial port. Additional
// options (such as EOP, tracing) can be configured through setters.
//
// Example
//
//	media := gxserial.NewGXSerial("COM1", gxserial.BaudRate9600, 8, gxserial.ParityNone, gxserial.StopBitsOne)
//
//	media.SetOnReceived(func(m IGXMedia, e ReceiveEventArgs) {
//	    // handle e.Data(), e.SenderInfo()
//	})
//	media.SetOnError(func(m IGXMedia, err error) {
//	    // log/handle error
//	})
//
//	if err := media.Open(); err != nil {
//	    // handle connect error
//	}
//	defer media.Close()
//
//	// send bytes; receive happens via the callback or via a blocking Receive.
//	_, _ = media.Send([]byte{0x01, 0x02, 0x03})
//
// # EOP framing
//
// When an EOP is configured, incoming bytes are buffered until the marker is
// observed. The marker can be a single byte (e.g. 0x7E), a string (e.g. "OK"),
// or an arbitrary byte slice. Disable EOP to read raw stream data.
//
// # Errors and timeouts
//
// Network and protocol errors are returned from calls or routed to Error
// handlers. Timeouts follow Go conventions (context/deadline or Duration-based
// configuration). Error messages are lowercased per Go style guidelines.
//
// # Notes
//
// The zero value of GXNet is not ready for use; always construct via NewGXNet.
// Long-running work in event handlers should be offloaded to a separate
// goroutine to avoid blocking I/O paths.
package gxserial
