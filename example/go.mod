module github.com/Gurux/gxserial-example-go

go 1.25.5

require (
	github.com/Gurux/gxcommon-go v1.0.16
	github.com/Gurux/gxserial-go v1.0.9
	golang.org/x/text v0.35.0
)

require golang.org/x/sys v0.42.0 // indirect

replace github.com/Gurux/gxserial-go => ../
