module github.com/cdalar/kubewarden-remove-cpu-limits

// Keep the go directive at 1.25: that is the newest Go release supported by the
// pinned TinyGo (0.39). A higher value makes `go` auto-download a toolchain
// TinyGo cannot use.
go 1.25

require (
	github.com/kubewarden/policy-sdk-go v0.13.1
	github.com/wapc/wapc-guest-tinygo v0.3.3
)

require (
	github.com/go-openapi/strfmt v0.21.3 // indirect
	github.com/kubewarden/k8s-objects v1.29.0-kw1 // indirect
)

replace github.com/go-openapi/strfmt => github.com/kubewarden/strfmt v0.1.3
