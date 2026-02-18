package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Gurux/gxcommon-go"
	"github.com/Gurux/gxserial-go"
	"golang.org/x/text/language"
)

var (
	port     = flag.String("S", "", "Port name")
	baudRate = flag.Int("b", 9600, "Baud rate")
	dataBits = flag.Int("d", 8, "DataBits (5, 6, 7, 8)")
	parity   = flag.String("p", "None", "Parity (None, Odd, Even, Mark, Space)")
	message  = flag.String("m", "", "Send message")
	t        = flag.String("t", "", "Trace level.")
	w        = flag.Int("w", 1000, "WaitTime in milliseconds.")
	lang     = flag.String("lang", "", "Used language.")
)

func main() {
	flag.Parse()
	if *port == "" || *message == "" {
		flag.PrintDefaults()
		return
	}

	br := gxcommon.BaudRate(*baudRate)
	Parity, err := gxcommon.ParityParse(*parity)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error parsing parity:", err)
		return
	}

	media := gxserial.NewGXSerial(*port, br, *dataBits, gxcommon.StopBitsOne, Parity)
	if *lang != "" {
		tag, err := language.Parse(*lang)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error parsing language:", err)
			return
		}
		media.Localize(tag)
	}

	media.SetOnError(func(m gxcommon.IGXMedia, err error) {
		// log/handle error
		fmt.Fprintln(os.Stderr, "error:", err)
	})

	media.SetOnReceived(func(m gxcommon.IGXMedia, e gxcommon.ReceiveEventArgs) {
		fmt.Printf("Async data: %s\n", e.String())
	})

	media.SetOnMediaStateChange(func(m gxcommon.IGXMedia, e gxcommon.MediaStateEventArgs) {
		fmt.Printf("Media state change : %s\n", e.State().String())
	})

	media.SetOnTrace(func(m gxcommon.IGXMedia, e gxcommon.TraceEventArgs) {
		fmt.Printf("Trace: %s\n", e.String())
	})

	err = media.Validate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return
	}

	if *t != "" {
		tl, err := gxcommon.TraceLevelParse(*t)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return
		}
		err = media.SetTrace(tl)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return
		}
	}
	if *t == "" {
		fmt.Printf("Trace level, %s!\n", *t)
	}
	fmt.Printf("Host port: %s\n", *port)
	fmt.Printf("Message: '%s'\n", *message)
	fmt.Printf("Trace level %s\n", media.GetTrace().String())
	err = media.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error returned:", err)
		ret, err := gxserial.GetPortNames()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to get available serial ports: ", err)
			return
		}
		fmt.Fprintln(os.Stderr, "Available serial ports: "+strings.Join(ret, ","))
		return
	}
	//Close the connection.
	defer func() {
		if err := media.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "close failed:", err)
		}
	}()

	//Send data synchronously.
	//Use defer media.GetSynchronous()() if sync is end when the method ends.
	//Or call media.GetSynchronous() when sync is needed and
	//call the returned function when sync is not needed anymore.
	func() {
		defer media.GetSynchronous()()
		err = media.Send(*message, "")
		//Send EOP
		err = media.Send("\n", "")
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
	}()
	fmt.Printf("Exit\n")
}
