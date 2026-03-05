See An [Gurux](http://www.gurux.org/ "Gurux") for an overview.

Join the Gurux Community or follow [@Gurux](https://twitter.com/guruxorg "@Gurux") for project updates.

With the gurux.serial component you can send data synchronously or
asynchronously over a serial port.

The open source `gxserial` media component (by Gurux Ltd) is part of the
GXMedias family.  It exposes a simple programmatic interface that works the
same across supported platforms:

* Windows
* Linux
* macOS

For complete API documentation refer to the GoDoc site
(https://pkg.go.dev/github.com/Gurux/gxserial-go) or browse the example in the
`example/` subdirectory.

Questions can be posted on the Gurux forum:
http://www.gurux.org/forum

Installation
============

```sh
go get github.com/Gurux/gxcommon-go
go get github.com/Gurux/gxserial-go
```

Quick start
===========

At a minimum you need to configure the port name and basic serial settings:

```go
media := gxserial.NewGXSerial("COM1", gxserial.BaudRate9600, 8,
    gxserial.ParityNone, gxserial.StopBitsOne)
```

Optional callbacks let you observe errors, received data, state changes and
trace messages:

```go
media.SetOnError(func(m gxcommon.IGXMedia, err error) {
    log.Println("serial error:", err)
})
media.SetOnReceived(func(m gxcommon.IGXMedia, e gxcommon.ReceiveEventArgs) {
    fmt.Printf("async data: %s\n", e.String())
})
```

Open the connection before sending or receiving:

```go
if err := media.Open(); err != nil {
    log.Fatal(err)
}
defer media.Close()
```

Sending is straightforward:

```go
_, _ = media.Send([]byte("Hello world"), "")
```

Receiving is normally asynchronous (via the callback above), but you can
enter synchronous mode with `media.GetSynchronous()` when implementing
request/response protocols.  See the example program for details.

Helper functions
----------------

- `gxserial.GetPortNames()` returns a slice of available port names on the
  current OS (`COM1`, `/dev/ttyUSB0`, etc.).

The API is documented on GoDoc; examples in this README are intentionally
minimal.

