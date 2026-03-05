package gxserial

import (
	"fmt"
)

// ExampleGetPortNames enumerates available serial ports.
func ExampleGetPortNames() {
	ports, err := GetPortNames()
	if err != nil {
		fmt.Println("failed:", err)
		return
	}
	fmt.Println(ports)

}
