package grpcclient_test

import (
	"context"
	"sync"
	"testing"
	"time"

	pb "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/api/grpcclient"
	"github.com/perfect-panel/ppanel-node/conf"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type concurrentControlServer struct {
	pb.UnimplementedNodeControlServiceServer

	mu       sync.Mutex
	metadata metadata.MD
	request  *pb.WatchControlRequest
	watched  chan struct{}
	reported chan struct{}
}

func (s *concurrentControlServer) WatchControl(req *pb.WatchControlRequest, stream grpc.ServerStreamingServer[pb.ControlEvent]) error {
	s.mu.Lock()
	s.metadata, _ = metadata.FromIncomingContext(stream.Context())
	s.request = req
	s.mu.Unlock()
	close(s.watched)
	select {
	case <-s.reported:
		return stream.Send(&pb.ControlEvent{
			EventId: "event-1",
			Payload: &pb.ControlEvent_ConfigChanged{ConfigChanged: &pb.ConfigChanged{}},
		})
	case <-stream.Context().Done():
		return stream.Context().Err()
	}
}

func (s *concurrentControlServer) ReportTelemetry(stream grpc.ClientStreamingServer[pb.TelemetryBatch, pb.ReportTelemetrySummary]) error {
	if _, err := stream.Recv(); err != nil {
		return err
	}
	close(s.reported)
	return stream.SendAndClose(&pb.ReportTelemetrySummary{})
}

func TestWatchControl_UsesAuthMetadataAndSharesConnectionWithTelemetry(t *testing.T) {
	server := &concurrentControlServer{
		watched:  make(chan struct{}),
		reported: make(chan struct{}),
	}
	addr, stop := startFakeServer(t, server)
	defer stop()

	client, err := grpcclient.New(&conf.ServerApiConfig{
		ServerId:        42,
		GRPCAddr:        addr,
		GRPCSecret:      "watch-secret",
		GRPCDialTimeout: 3,
		GRPCRPCTimeout:  1,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := &pb.WatchControlRequest{
		ServerId:       42,
		Protocols:      []string{"trojan", "vless"},
		NodeInstanceId: "instance-1",
		NodeVersion:    "v1.2.3",
	}
	stream, err := client.WatchControl(ctx, request)
	require.NoError(t, err)
	select {
	case <-server.watched:
	case <-time.After(3 * time.Second):
		t.Fatal("WatchControl did not reach server")
	}

	// Exceed the configured unary RPC timeout to prove the watch stream has no RPC deadline.
	time.Sleep(1200 * time.Millisecond)
	summary, err := client.ReportTelemetry(context.Background(), &pb.TelemetryBatch{})
	require.NoError(t, err)
	require.NotNil(t, summary)

	event, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, event.GetConfigChanged())

	server.mu.Lock()
	defer server.mu.Unlock()
	require.Equal(t, "watch-secret", server.metadata.Get("x-node-secret")[0])
	require.Equal(t, "42", server.metadata.Get("x-node-id")[0])
	require.True(t, proto.Equal(request, server.request))
}
