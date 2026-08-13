module appforge/proto/system

go 1.26.4

require (
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
	appforge/proto/common v0.0.0-00010101000000-000000000000
)

require (
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260727163830-6c54dddc4772 // indirect
)

replace appforge/proto/common => ../common
