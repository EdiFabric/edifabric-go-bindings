module github.com/EdiFabric/edifabric-go-bindings/native

go 1.25.0

require github.com/EdiFabric/edifabric-go-bindings v1.1.1

require github.com/ebitengine/purego v0.11.1 // indirect

// Builds inside this repository use the bindings beside it; modules that depend on
// this one ignore the replace and use the version above.
replace github.com/EdiFabric/edifabric-go-bindings => ../
