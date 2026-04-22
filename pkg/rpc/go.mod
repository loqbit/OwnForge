module github.com/ownforge/ownforge/pkg/rpc

go 1.25.5

require (
	github.com/ownforge/ownforge/pkg/proto v1.3.0
	google.golang.org/grpc v1.79.3
)

require (
	go.opentelemetry.io/otel/metric v1.42.0 // indirect
	go.opentelemetry.io/otel/sdk v1.42.0 // indirect
	go.opentelemetry.io/otel/trace v1.42.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/ownforge/ownforge/pkg/proto => ../proto
