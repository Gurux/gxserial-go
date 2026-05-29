module github.com/Gurux/gxserial-example-go

go 1.25.5

require (
	github.com/Gurux/gxcommon-go v1.0.20
	github.com/Gurux/gxserial-go v1.0.11
	golang.org/x/text v0.37.0
)

require golang.org/x/sys v0.45.0 // indirect

replace github.com/Gurux/gxserial-go => ../
