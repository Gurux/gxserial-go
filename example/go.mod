module github.com/Gurux/gxserial-example-go

go 1.25.5

require (
	github.com/Gurux/gxcommon-go v1.0.21
	github.com/Gurux/gxserial-go v1.0.13
	golang.org/x/text v0.41.0
)

require golang.org/x/sys v0.47.0 // indirect

replace github.com/Gurux/gxserial-go => ../
