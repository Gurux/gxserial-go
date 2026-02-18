module github.com/Gurux/gxserial-example-go

go 1.25.5

require (
	github.com/Gurux/gxcommon-go v1.0.8
	github.com/Gurux/gxserial-go v1.0.1
	golang.org/x/text v0.34.0
)

require golang.org/x/sys v0.41.0 // indirect

replace github.com/Gurux/gxserial-go => ../
