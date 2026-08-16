module appforge/proto/builder

go 1.26.6

require (
	appforge/proto/common v0.0.0-00010101000000-000000000000
	appforge/proto/core v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
)

replace appforge/proto/common => ../common

replace appforge/proto/core => ../core
