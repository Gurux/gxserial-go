See An [Gurux](http://www.gurux.org/ "Gurux") for an overview.

Join the Gurux Community or follow [@Gurux](https://twitter.com/guruxorg "@Gurux") for project updates.

With gurux.serial component you can send data easily syncronously or asyncronously using serial port connection.

Open Source gxserial media component, made by Gurux Ltd, is a part of GXMedias set of media components, which programming interfaces help you implement communication by chosen connection type. Gurux media components also support the following connection types: serial port.

For more info check out [Gurux](http://www.gurux.org/ "Gurux").

We are updating documentation on Gurux web page. 

If you have problems you can ask your questions in Gurux [Forum](http://www.gurux.org/forum).

You can get source codes from http://www.github.com/gurux or add reference to your project:

```go
go get github.com/Gurux/gxcommon-go
go get github.com/Gurux/gxserial-go
```

Simple example
=========================== 
Before use you must set following settings:
* PortName
* BaudRate
* DataBits
* Parity
* StopBits

It is also good to add listener to listen following events.
* onError
* onReceived
* onMediaStateChange
* onTrace
* onPropertyChanged

```go
media := gxserial.NewGXSerial("COM1", gxserial.BaudRate9600, 8, gxserial.ParityNone, gxserial.StopBitsOne)
media.open()
```

Data is send with send command:

```go
media.Send("Hello World!", "")
```
In default mode received data is coming as asynchronously from OnReceived event.
Event listener is added like this:
```go
media.SetOnReceived(func(m gxcommon.IGXMedia, e gxcommon.ReceiveEventArgs) {
	fmt.Printf("Async data: %s\n", e.String())
})

```
Data can be also send as syncronous if needed.
```go
defer media.GetSynchronous()()
err = media.Send("Hello World!\n", "")
if err != nil {
    fmt.Fprintln(os.Stderr, "error:", err)
    return
}
r := gxcommon.NewReceiveParameters[string]()
r.EOP = "\n"
r.WaitTime = *w
r.Count = 0
ret, err := media.Receive(r)
if err != nil {
    fmt.Fprintln(os.Stderr, "error returned:", err)
    return
}
if ret {
    fmt.Printf("Sync data: %s\n", r.Reply)
}
```
