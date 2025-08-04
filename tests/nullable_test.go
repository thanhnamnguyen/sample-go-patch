package tests

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestNullable(t *testing.T) {
	// encoding.RegisterCodec(vt.Codec{})

	server, err := testServer()
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
		return
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
		return
	}

	go func() {
		server.Serve(lis)
	}()

	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()
	client := NewTestServiceClient(conn)
	now := time.Now()
	fmt.Printf("now: %v\n", now)
	req := &Test{
		NullableName: NullableType{
			Name: now.Format(time.RFC3339),
		},
	}
	resp, err := client.GetTest(context.Background(), req)
	if err != nil {
		log.Fatalf("failed to get test: %v", err)
	}
	fmt.Printf("GetTest: %v", resp.GetNullableName())
}

// create a server
func testServer() (*grpc.Server, error) {
	// create a server
	server := grpc.NewServer()
	RegisterTestServiceServer(server, &TestService{})

	return server, nil
}

type TestService struct {
	UnimplementedTestServiceServer
}

func (s *TestService) GetTest(ctx context.Context, req *Test) (*Test, error) {
	log.Printf("GetTest: %v", req.NullableName)
	return req, nil
}
