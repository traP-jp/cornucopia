module github.com/traP-jp/plutus/system/cornucopia

go 1.25.5

replace github.com/traP-jp/plutus/api/protobuf => ./api/protobuf

require (
	github.com/go-sql-driver/mysql v1.9.3
	github.com/google/uuid v1.6.0
	github.com/traP-jp/plutus/api/protobuf v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.77.0
)

require (
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/pressly/goose/v3 v3.26.0 // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	google.golang.org/protobuf v1.36.11
)
