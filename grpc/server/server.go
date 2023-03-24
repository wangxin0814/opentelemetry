package main

import (
	"context"
	"fmt"
	"github.com/wangxin0814/opentelemetry/grpc/api"
	"github.com/wangxin0814/opentelemetry/grpc/config"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"time"
)

const (
	port = ":7777"
)

// server is used to implement api.HelloServiceServer.
type server struct {
	api.HelloServiceServer
}

// SayHello implements api.HelloServiceServer.
func (s *server) SayHello(ctx context.Context, in *api.HelloRequest) (*api.HelloResponse, error) {
	log.Printf("Received: %v\n", in.GetGreeting())
	s.workHard(ctx)
	time.Sleep(50 * time.Millisecond)

	return &api.HelloResponse{Reply: "Hello " + in.Greeting}, nil
}

func (s *server) workHard(ctx context.Context) {
	tracer := config.TraceFromContext(ctx)

	_, span := tracer.Start(ctx, "workHard",
		trace.WithAttributes(attribute.String("extra.key", "extra.value")))
	defer span.End()

	log.Printf("trace_id: %s, span_id: %s", span.SpanContext().TraceID().String(), span.SpanContext().SpanID().String())

	time.Sleep(50 * time.Millisecond)
}

func (s *server) SayHelloServerStream(in *api.HelloRequest, out api.HelloService_SayHelloServerStreamServer) error {
	log.Printf("Received: %v\n", in.GetGreeting())

	for i := 0; i < 5; i++ {
		err := out.Send(&api.HelloResponse{Reply: "Hello " + in.Greeting})
		if err != nil {
			return err
		}

		time.Sleep(time.Duration(i*50) * time.Millisecond)
	}

	return nil
}

func (s *server) SayHelloClientStream(stream api.HelloService_SayHelloClientStreamServer) error {
	i := 0

	for {
		in, err := stream.Recv()

		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("Non EOF error: %v\n", err)
			return err
		}

		log.Printf("Received: %v\n", in.GetGreeting())
		i++
	}

	time.Sleep(50 * time.Millisecond)

	return stream.SendAndClose(&api.HelloResponse{Reply: fmt.Sprintf("Hello (%v times)", i)})
}

func (s *server) SayHelloBidiStream(stream api.HelloService_SayHelloBidiStreamServer) error {
	for {
		in, err := stream.Recv()

		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("Non EOF error: %v\n", err)
			return err
		}

		time.Sleep(50 * time.Millisecond)

		log.Printf("Received: %v\n", in.GetGreeting())
		err = stream.Send(&api.HelloResponse{Reply: "Hello " + in.Greeting})

		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	tp, err := config.Init("grpc-server")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),   // 一元拦截器
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()), // 流式拦截器
	)

	api.RegisterHelloServiceServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
