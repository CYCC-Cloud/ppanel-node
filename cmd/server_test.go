package cmd

import (
	"errors"
	"testing"

	pb "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/api/grpcclient"
	"github.com/perfect-panel/ppanel-node/conf"
	"github.com/stretchr/testify/require"
)

func TestReload_PreflightFailurePreservesCurrentRuntime(t *testing.T) {
	originalPrepare := prepareRuntimeForReload
	prepareRuntimeForReload = func(string) (*conf.Conf, *grpcclient.Client, *pb.GetConfigResponse, string, error) {
		return nil, nil, nil, "", errors.New("preflight failed")
	}
	t.Cleanup(func() { prepareRuntimeForReload = originalPrepare })

	old := &serverRuntime{}
	current := old
	err := reload("ignored.yml", &current, make(chan struct{}, 1), "process-uuid")
	require.EqualError(t, err, "preflight failed")
	require.Same(t, old, current)
}

func TestReload_CutoverFailureReturnsAndClearsRuntime(t *testing.T) {
	originalPrepare := prepareRuntimeForReload
	originalStart := startRuntimeForReload
	t.Cleanup(func() {
		prepareRuntimeForReload = originalPrepare
		startRuntimeForReload = originalStart
	})

	newClient := &grpcclient.Client{}
	prepareRuntimeForReload = func(string) (*conf.Conf, *grpcclient.Client, *pb.GetConfigResponse, string, error) {
		return conf.New(), newClient, &pb.GetConfigResponse{Data: &pb.ServerConfig{}}, "rev-2", nil
	}
	var receivedInstanceID string
	startRuntimeForReload = func(_ *conf.Conf, client *grpcclient.Client, _ *pb.GetConfigResponse, _ string, _ chan struct{}, nodeInstanceID string) (*serverRuntime, error) {
		require.Same(t, newClient, client)
		receivedInstanceID = nodeInstanceID
		_ = client.Close()
		return nil, errors.New("new runtime failed")
	}

	old := &serverRuntime{client: &grpcclient.Client{}}
	current := old
	err := reload("ignored.yml", &current, make(chan struct{}, 1), "process-uuid")
	require.EqualError(t, err, "new runtime failed")
	require.Nil(t, current)
	require.Nil(t, old.client)
	require.Equal(t, "process-uuid", receivedInstanceID)
}

func TestProtocolTypes_DeduplicatesAndSorts(t *testing.T) {
	response := &pb.GetConfigResponse{Data: &pb.ServerConfig{Protocols: []*pb.Protocol{
		{Type: "vless"},
		{Type: "trojan"},
		{Type: "vless"},
		{Type: ""},
	}}}
	require.Equal(t, []string{"trojan", "vless"}, protocolTypes(response))
}

func TestWatchControlRequest_ContainsRuntimeIdentityAndProtocols(t *testing.T) {
	originalVersion := version
	version = "v9.8.7"
	t.Cleanup(func() { version = originalVersion })

	c := conf.New()
	c.ApiConfig.ServerId = 42
	response := &pb.GetConfigResponse{Data: &pb.ServerConfig{Protocols: []*pb.Protocol{
		{Type: "vless"},
		{Type: "trojan"},
		{Type: "vless"},
	}}}
	request := watchControlRequest(c, response, "process-uuid")
	require.Equal(t, int64(42), request.GetServerId())
	require.Equal(t, []string{"trojan", "vless"}, request.GetProtocols())
	require.Equal(t, "process-uuid", request.GetNodeInstanceId())
	require.Equal(t, "v9.8.7", request.GetNodeVersion())
}

func TestServerRuntime_CloseIsNilSafeAndIdempotent(t *testing.T) {
	var nilRuntime *serverRuntime
	require.NoError(t, nilRuntime.Close())

	runtime := &serverRuntime{client: &grpcclient.Client{}}
	require.NoError(t, runtime.Close())
	require.NoError(t, runtime.Close())
	require.Nil(t, runtime.client)
}
