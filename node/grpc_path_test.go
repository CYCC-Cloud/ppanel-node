package node

import (
	"testing"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/api/panel"
	vCore "github.com/perfect-panel/ppanel-node/core"
	"github.com/perfect-panel/ppanel-node/limiter"
	"github.com/stretchr/testify/require"
)

// TestNoHTTPControlPlaneCallsInRuntimePath verifies that when gRPC clients are
// configured (userClient + telemetryClient set, apiClient nil), Controller.Start
// and the periodic tasks do NOT panic or require a live HTTP control-plane.
func TestNoHTTPControlPlaneCallsInRuntimePath(t *testing.T) {
	limiter.Init()

	users := []*nodecontrolv1.ServerUser{
		{Id: 1, Uuid: "grpc-user-1"},
	}
	userClient := &fakeUserListClient{
		response: &nodecontrolv1.GetUserListResponse{
			Changed:  true,
			Revision: "rev1",
			Users:    users,
		},
	}
	telClient := &fakeTelemetryClient{}
	srv := &fakeCoreServer{}

	nodeInfo := &panel.NodeInfo{
		Id:           1,
		Type:         "vmess",
		PushInterval: 60,
		PullInterval: 60,
		Protocol: &panel.Protocol{
			Type: "vmess",
		},
	}

	// NewController called with nil apiClient — as it will be in gRPC-only mode.
	c := NewController(srv, nil, "grpc-host:50051", userClient, telClient, nodeInfo)

	// Start should succeed without an HTTP client: it must use gRPC userClient only.
	err := c.Start()
	require.NoError(t, err, "Start() must succeed with nil apiClient in gRPC mode")

	// Cleanup
	c.Close()
}

// fakeCoreServerForGRPCPath satisfies coreServer without xray machinery.
var _ coreServer = (*fakeCoreServerForGRPCPath)(nil)

type fakeCoreServerForGRPCPath struct{}

func (f *fakeCoreServerForGRPCPath) AddNode(_ string, _ *panel.NodeInfo) error { return nil }
func (f *fakeCoreServerForGRPCPath) DelNode(_ string) error                    { return nil }
func (f *fakeCoreServerForGRPCPath) AddUsers(_ *vCore.AddUsersParams) (int, error) {
	return 1, nil
}
func (f *fakeCoreServerForGRPCPath) DelUsers(_ []panel.UserInfo, _ string, _ *panel.NodeInfo) error {
	return nil
}
func (f *fakeCoreServerForGRPCPath) GetUserTrafficSlice(_ string, _ int) ([]panel.UserTraffic, error) {
	return nil, nil
}
