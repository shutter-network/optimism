package shutternode

//go:generate protoc --proto_path=./proto --go_out=./grpc --go_opt=paths=source_relative --go-grpc_out=./grpc --go-grpc_opt=paths=source_relative ./proto/v1/service.proto
