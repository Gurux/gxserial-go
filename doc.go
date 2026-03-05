// Package gxserial provides a cross‑platform serial‑port media implementation
// used by Gurux components (see https://www.gurux.org).  It satisfies the
// gxcommon.IGXMedia interface and exposes a simple, Go‑idiomatic API for
// opening a connection, sending and receiving data, and subscribing to events
// such as received bytes, trace information, errors and state changes.
//
// The primary exported type is *GXSerial, which wraps platform‑specific
// drivers (Windows, Linux, macOS) to present a consistent configuration surface
// and built‑in helpers such as byte counters, framing support and synchronous
// receive semantics.
//
// # Features
//
//   * Configurable serial settings (port name, baud rate, data bits, parity,
//     stop bits).
//   * Asynchronous callbacks and optional synchronous request/response mode.
//   * Framing: optional end‑of‑packet (EOP) marker defined as a byte, string or
//     arbitrary []byte.  Turning off EOP gives raw stream access.
//   * Timeouts: connection and I/O timeouts expressed as time.Duration values.
//   * Tracing: configurable trace level and mask for sent/received/error/info
//     messages.
//   * Events: Received, Error, Trace and MediaState handlers; callbacks are
//     safe for concurrent use.
//   * Concurrency: safe for concurrent reads and writes; Close unblocks pending
//     I/O operations.
//   * Helpers such as GetPortNames to enumerate available serial ports on the
//     host operating system.
//
// # Usage
//
// Create a new connection with NewGXSerial and then configure any optional
// settings before calling Open.  Most examples use asynchronous receive via a
// callback, but the library also supports a synchronous mode useful for simple
// request/response protocols.
//
// Example:
//
//	media := gxserial.NewGXSerial("COM1", gxserial.BaudRate9600, 8, gxserial.ParityNone, gxserial.StopBitsOne)
//
//	media.SetOnReceived(func(m gxserial.IGXMedia, e gxserial.ReceiveEventArgs) {
//	    // handle e.Data(), e.SenderInfo()
//	})
//	media.SetOnError(func(m gxserial.IGXMedia, err error) {
//	    // log or handle error
//	})
//
//	if err := media.Open(); err != nil {
//	    // handle connect error
//	}
//	defer media.Close()
//
//	// send bytes; receive happens via the callback or via blocking Receive.
//	_, _ = media.Send([]byte{0x01, 0x02, 0x03})
//
// # Additional helpers
//
// GetPortNames returns a slice of port names valid for the current platform
// ("COM1"/"COM2" on Windows, "/dev/ttyUSB0" etc. on Unix‑like systems).
//
// # Referencing examples
//
// The repository contains a runnable example under the example/ directory as
// well as Go examples in the package tests that appear in godoc output.
//
// # Notes
//
// The zero value of *GXSerial is not usable; always obtain an instance via
// NewGXSerial.  Long‑running work in event handlers should be dispatched to a
// separate goroutine to avoid blocking I/O paths.
package gxserial
