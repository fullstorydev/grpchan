module github.com/fullstorydev/grpchan

go 1.22.7

replace google.golang.org/grpc => github.com/dfawley/grpc-go v1.49.0-dev.0.20241101162700-fcd45dd4befc

require (
	github.com/golang/protobuf v1.5.4
	github.com/jhump/gopoet v0.1.0
	github.com/jhump/goprotoc v0.5.0
	github.com/jhump/protoreflect v1.15.6
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241015192408-796eee8c2d53
	google.golang.org/grpc v1.67.1
	google.golang.org/protobuf v1.35.1
)

require (
	github.com/bufbuild/protocompile v0.9.0 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
)
