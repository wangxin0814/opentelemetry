//go:generate protoc -I=. -I=$GOPATH/src --go_out=plugins=grpc:. api/hello-service.proto

package api
