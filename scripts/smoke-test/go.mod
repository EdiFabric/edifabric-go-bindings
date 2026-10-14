module github.com/EdiFabric/edifabric-go-bindings/scripts/smoke-test

go 1.25.0

require (
	github.com/EdiFabric/edifabric-go-bindings v1.1.1
	github.com/EdiFabric/edifabric-go-bindings/native v1.1.1
)

require github.com/ebitengine/purego v0.11.1 // indirect

replace (
	github.com/EdiFabric/edifabric-go-bindings => ../../
	github.com/EdiFabric/edifabric-go-bindings/native => ../../native
)
