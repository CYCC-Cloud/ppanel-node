package cmd

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/api/grpcclient"
	"github.com/perfect-panel/ppanel-node/conf"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type sliceControlReceiver struct {
	events []*pb.ControlEvent
	index  int
}

func (r *sliceControlReceiver) Recv() (*pb.ControlEvent, error) {
	if r.index == len(r.events) {
		return nil, errors.New("stream closed")
	}
	event := r.events[r.index]
	r.index++
	return event, nil
}

func TestControlWatcher_ConfigSignalsMergeAndNeverBlock(t *testing.T) {
	reloadCh := make(chan struct{}, 1)
	receiver := &sliceControlReceiver{events: []*pb.ControlEvent{
		configChangedEvent("event-1"),
		configChangedEvent("event-2"),
	}}
	done := make(chan error, 1)
	go func() {
		done <- receiveControlEvents(context.Background(), receiver, reloadCh)
	}()

	select {
	case err := <-done:
		require.EqualError(t, err, "stream closed")
	case <-time.After(time.Second):
		t.Fatal("control event receiver blocked on a full reload channel")
	}
	require.Len(t, reloadCh, 1)
}

type watcherTestServer struct {
	pb.UnimplementedNodeControlServiceServer
	watch func(*pb.WatchControlRequest, grpc.ServerStreamingServer[pb.ControlEvent]) error
}

func (s *watcherTestServer) WatchControl(req *pb.WatchControlRequest, stream grpc.ServerStreamingServer[pb.ControlEvent]) error {
	return s.watch(req, stream)
}

func TestControlWatcher_RetriesUnavailableAndPreservesRequest(t *testing.T) {
	originalDelay := controlWatchRetryDelay
	controlWatchRetryDelay = func(int) time.Duration { return time.Millisecond }
	t.Cleanup(func() { controlWatchRetryDelay = originalDelay })

	var attempts atomic.Int32
	requestSeen := make(chan *pb.WatchControlRequest, 1)
	client, stop := startWatcherTestClient(t, &watcherTestServer{watch: func(req *pb.WatchControlRequest, stream grpc.ServerStreamingServer[pb.ControlEvent]) error {
		if attempts.Add(1) == 1 {
			return status.Error(codes.Unavailable, "temporary outage")
		}
		requestSeen <- req
		return stream.Context().Err()
	}})
	defer stop()

	req := &pb.WatchControlRequest{ServerId: 42, Protocols: []string{"trojan", "vless"}, NodeInstanceId: "process-uuid", NodeVersion: "v2"}
	watcher := startControlWatcher(client, req, make(chan struct{}, 1))
	defer watcher.Stop()

	select {
	case actual := <-requestSeen:
		require.Equal(t, req.GetServerId(), actual.GetServerId())
		require.Equal(t, req.GetProtocols(), actual.GetProtocols())
		require.Equal(t, req.GetNodeInstanceId(), actual.GetNodeInstanceId())
		require.Equal(t, req.GetNodeVersion(), actual.GetNodeVersion())
	case <-time.After(3 * time.Second):
		t.Fatal("WatchControl did not reconnect after Unavailable")
	}
	require.GreaterOrEqual(t, attempts.Load(), int32(2))
}

func TestControlWatcher_UnimplementedStopsWithoutRetry(t *testing.T) {
	originalDelay := controlWatchRetryDelay
	controlWatchRetryDelay = func(int) time.Duration { return time.Millisecond }
	t.Cleanup(func() { controlWatchRetryDelay = originalDelay })

	var attempts atomic.Int32
	client, stop := startWatcherTestClient(t, &watcherTestServer{watch: func(_ *pb.WatchControlRequest, _ grpc.ServerStreamingServer[pb.ControlEvent]) error {
		attempts.Add(1)
		return status.Error(codes.Unimplemented, "not supported")
	}})
	defer stop()

	watcher := startControlWatcher(client, &pb.WatchControlRequest{ServerId: 42}, make(chan struct{}, 1))
	select {
	case <-watcher.done:
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not stop after Unimplemented")
	}
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, int32(1), attempts.Load())
	watcher.Stop()
}

func TestControlWatcher_StopCancelsActiveStream(t *testing.T) {
	connected := make(chan struct{})
	client, stop := startWatcherTestClient(t, &watcherTestServer{watch: func(_ *pb.WatchControlRequest, stream grpc.ServerStreamingServer[pb.ControlEvent]) error {
		close(connected)
		<-stream.Context().Done()
		return stream.Context().Err()
	}})
	defer stop()

	watcher := startControlWatcher(client, &pb.WatchControlRequest{ServerId: 42}, make(chan struct{}, 1))
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("WatchControl stream was not established")
	}

	stopped := make(chan struct{})
	go func() {
		watcher.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("watcher Stop did not cancel the active stream")
	}
}

func startWatcherTestClient(t *testing.T, impl pb.NodeControlServiceServer) (*grpcclient.Client, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	pb.RegisterNodeControlServiceServer(server, impl)
	go func() { _ = server.Serve(listener) }()

	client, err := grpcclient.New(&conf.ServerApiConfig{
		ServerId:        42,
		GRPCAddr:        listener.Addr().String(),
		GRPCSecret:      "secret",
		GRPCDialTimeout: 3,
		GRPCRPCTimeout:  1,
	})
	require.NoError(t, err)
	return client, func() {
		_ = client.Close()
		server.Stop()
		_ = listener.Close()
	}
}

func configChangedEvent(id string) *pb.ControlEvent {
	return &pb.ControlEvent{
		EventId: id,
		Payload: &pb.ControlEvent_ConfigChanged{ConfigChanged: &pb.ConfigChanged{}},
	}
}
