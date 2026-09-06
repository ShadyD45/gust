package otel

import (
	"context"
	"net"
	"sync"

	colltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcState struct {
	mu     sync.Mutex
	server *grpc.Server
	ln     net.Listener
	addr   string
}

func (r *Receiver) startGRPC() error {
	ln, err := net.Listen("tcp", r.cfg.GRPCListen)
	if err != nil {
		return err
	}
	srv := grpc.NewServer()
	colltrace.RegisterTraceServiceServer(srv, &traceService{receiver: r})
	r.grpc = &grpcState{server: srv, ln: ln, addr: ln.Addr().String()}
	go func() { _ = srv.Serve(ln) }()
	return nil
}

// GRPCEndpoint returns the bound gRPC host:port, or empty if gRPC is off.
func (r *Receiver) GRPCEndpoint() string {
	if r.grpc == nil {
		return ""
	}
	return r.grpc.addr
}

func (r *Receiver) stopGRPC() {
	if r.grpc == nil {
		return
	}
	r.grpc.mu.Lock()
	defer r.grpc.mu.Unlock()
	if r.grpc.server != nil {
		r.grpc.server.GracefulStop()
	}
}

type traceService struct {
	colltrace.UnimplementedTraceServiceServer
	receiver *Receiver
}

func (s *traceService) Export(ctx context.Context, req *colltrace.ExportTraceServiceRequest) (*colltrace.ExportTraceServiceResponse, error) {
	_ = ctx
	if err := s.receiver.Ingest(payloadFromProtoRequest(req)); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &colltrace.ExportTraceServiceResponse{}, nil
}
