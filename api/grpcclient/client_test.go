package grpcclient_test

import (
	"context"
	"net"
	"testing"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/api/grpcclient"
	"github.com/perfect-panel/ppanel-node/conf"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeNodeControlServer struct {
	nodecontrolv1.UnimplementedNodeControlServiceServer
	getConfigFn func(ctx context.Context, req *nodecontrolv1.GetConfigRequest) (*nodecontrolv1.GetConfigResponse, error)
}

func (f *fakeNodeControlServer) GetConfig(ctx context.Context, req *nodecontrolv1.GetConfigRequest) (*nodecontrolv1.GetConfigResponse, error) {
	if f.getConfigFn != nil {
		return f.getConfigFn(ctx, req)
	}
	return &nodecontrolv1.GetConfigResponse{Changed: true, Data: &nodecontrolv1.ServerConfig{}}, nil
}

func TestAuthMetadataInjected(t *testing.T) {
	var captured metadata.MD
	srvAddr, stop := startFakeServer(t, &fakeNodeControlServer{
		getConfigFn: func(ctx context.Context, req *nodecontrolv1.GetConfigRequest) (*nodecontrolv1.GetConfigResponse, error) {
			captured, _ = metadata.FromIncomingContext(ctx)
			return &nodecontrolv1.GetConfigResponse{Changed: true, Data: &nodecontrolv1.ServerConfig{}}, nil
		},
	})
	defer stop()

	c, err := grpcclient.New(&conf.ServerApiConfig{
		ServerId:        42,
		Transport:       "grpc",
		GRPCAddr:        srvAddr,
		GRPCSecret:      "node-secret",
		GRPCDialTimeout: 3,
		GRPCRPCTimeout:  3,
		GRPCTLS:         false,
		GRPCServerName:  "",
		SecretKey:       "",
		ApiHost:         "",
		Timeout:         0,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = c.Close()
	})

	_, err = c.GetConfig(context.Background(), "", []string{"vless"})
	require.NoError(t, err)
	require.Equal(t, "node-secret", captured.Get("x-node-secret")[0])
	require.Equal(t, "42", captured.Get("x-node-id")[0])
}

func TestGetConfig_ChangedFalseReturnsNilData(t *testing.T) {
	srvAddr, stop := startFakeServer(t, &fakeNodeControlServer{
		getConfigFn: func(ctx context.Context, req *nodecontrolv1.GetConfigRequest) (*nodecontrolv1.GetConfigResponse, error) {
			return &nodecontrolv1.GetConfigResponse{Changed: false, Revision: "r2", Data: nil}, nil
		},
	})
	defer stop()

	c, err := grpcclient.New(&conf.ServerApiConfig{
		ServerId:        7,
		Transport:       "grpc",
		GRPCAddr:        srvAddr,
		GRPCSecret:      "node-secret",
		GRPCDialTimeout: 3,
		GRPCRPCTimeout:  3,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = c.Close()
	})

	resp, err := c.GetConfig(context.Background(), "known", []string{"vless"})
	require.NoError(t, err)
	require.Nil(t, resp)
}

func startFakeServer(t *testing.T, impl nodecontrolv1.NodeControlServiceServer) (string, func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := grpc.NewServer()
	nodecontrolv1.RegisterNodeControlServiceServer(s, impl)

	go func() {
		_ = s.Serve(lis)
	}()

	return lis.Addr().String(), func() {
		s.Stop()
		_ = lis.Close()
	}
}
