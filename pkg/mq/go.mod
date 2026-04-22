module github.com/ownforge/ownforge/pkg/mq

go 1.25.5

require (
	github.com/nats-io/nats.go v1.50.0
	github.com/ownforge/ownforge/pkg/trace v0.1.0
	github.com/segmentio/kafka-go v0.4.50
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/nats-io/nkeys v0.4.15 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	go.opentelemetry.io/otel v1.42.0 // indirect
	go.opentelemetry.io/otel/trace v1.42.0 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
)

replace github.com/ownforge/ownforge/pkg/trace => ../trace
