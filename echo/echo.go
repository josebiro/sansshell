package echo

import (
	context "context"

	"google.golang.org/grpc"
)

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative echo.proto

// server is used to implement the gRPC server
type Server struct{}

func (s *Server) Echo(ctx context.Context, req *EchoRequest) (*EchoResponse, error) {
	return &EchoResponse{
		Output: req.Input,
	}, nil
}

// Install is called to expose this handler to the gRPC server
func (s *Server) Register(gs *grpc.Server) {
	RegisterEchoServiceServer(gs, s)
}
