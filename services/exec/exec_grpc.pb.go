// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package exec

import (
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ExecClient is the client API for Exec service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ExecClient interface {
	// Exec takes input, executes it and returns result of input execution
	Exec(ctx context.Context, in *ExecRequest, opts ...grpc.CallOption) (*ExecResponse, error)
}

type execClient struct {
	cc grpc.ClientConnInterface
}

func NewExecClient(cc grpc.ClientConnInterface) ExecClient {
	return &execClient{cc}
}

func (c *execClient) Exec(ctx context.Context, in *ExecRequest, opts ...grpc.CallOption) (*ExecResponse, error) {
	out := new(ExecResponse)
	err := c.cc.Invoke(ctx, "/Exec/Exec", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ExecServer is the server API for Exec service.
// All implementations should embed UnimplementedExecServer
// for forward compatibility
type ExecServer interface {
	// Exec takes input, executes it and returns result of input execution
	Exec(context.Context, *ExecRequest) (*ExecResponse, error)
}

// UnimplementedExecServer should be embedded to have forward compatible implementations.
type UnimplementedExecServer struct {
}

func (UnimplementedExecServer) Exec(context.Context, *ExecRequest) (*ExecResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Exec not implemented")
}

// UnsafeExecServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ExecServer will
// result in compilation errors.
type UnsafeExecServer interface {
	mustEmbedUnimplementedExecServer()
}

func RegisterExecServer(s grpc.ServiceRegistrar, srv ExecServer) {
	s.RegisterService(&Exec_ServiceDesc, srv)
}

func _Exec_Exec_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ExecRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ExecServer).Exec(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Exec/Exec",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ExecServer).Exec(ctx, req.(*ExecRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Exec_ServiceDesc is the grpc.ServiceDesc for Exec service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Exec_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Exec",
	HandlerType: (*ExecServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Exec",
			Handler:    _Exec_Exec_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "exec.proto",
}
