package node

import (
	"context"
	"testing"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	vCore "github.com/perfect-panel/ppanel-node/core"
	"github.com/perfect-panel/ppanel-node/domain"
	"github.com/perfect-panel/ppanel-node/limiter"
	"github.com/stretchr/testify/require"
)

type fakeCoreServer struct {
	added   []*vCore.AddUsersParams
	deleted [][]domain.UserInfo
}

func (f *fakeCoreServer) AddNode(tag string, info *domain.NodeInfo) error { return nil }
func (f *fakeCoreServer) DelNode(tag string) error                        { return nil }
func (f *fakeCoreServer) AddUsers(params *vCore.AddUsersParams) (int, error) {
	f.added = append(f.added, params)
	return len(params.Users), nil
}
func (f *fakeCoreServer) DelUsers(users []domain.UserInfo, tag string, info *domain.NodeInfo) error {
	f.deleted = append(f.deleted, users)
	return nil
}
func (f *fakeCoreServer) GetUserTrafficSlice(tag string, mintraffic int) ([]domain.UserTraffic, error) {
	return nil, nil
}

type fakeUserListClient struct {
	response *nodecontrolv1.GetUserListResponse
	err      error
	calls    int
}

func (f *fakeUserListClient) GetUserList(ctx context.Context, protocol, knownRevision string) (*nodecontrolv1.GetUserListResponse, error) {
	f.calls++
	return f.response, f.err
}

func newTestController(t *testing.T, initial []domain.UserInfo, userClient userListClient) (*Controller, *fakeCoreServer) {
	t.Helper()
	limiter.Init()
	server := &fakeCoreServer{}
	controller := &Controller{
		server:     server,
		apiHost:    "test-host",
		userClient: userClient,
		tag:        "test",
		info: &domain.NodeInfo{
			Id: 1,
			Protocol: &domain.Protocol{
				Type: "vmess",
			},
		},
		userList: initial,
	}
	controller.limiter = limiter.AddLimiter(controller.tag, initial, nil)
	return controller, server
}

func TestUserListMonitor_ChangedUpdatesUsers(t *testing.T) {
	initial := []domain.UserInfo{{Id: 1, Uuid: "old", SpeedLimit: 0, DeviceLimit: 0}}
	userClient := &fakeUserListClient{
		response: &nodecontrolv1.GetUserListResponse{
			Changed:  true,
			Revision: "rev2",
			Users: []*nodecontrolv1.ServerUser{
				{Id: 2, Uuid: "new", SpeedLimit: 10, DeviceLimit: 2},
			},
		},
	}
	controller, server := newTestController(t, initial, userClient)

	err := controller.userListMonitor()
	require.NoError(t, err)
	require.Equal(t, 1, userClient.calls)
	require.Len(t, server.deleted, 1)
	require.Len(t, server.deleted[0], 1)
	require.Equal(t, initial, server.deleted[0])
	require.Len(t, server.added, 1)
	require.Len(t, server.added[0].Users, 1)
	require.Equal(t, "new", server.added[0].Users[0].Uuid)
	require.Equal(t, "rev2", controller.knownRevision)
	_, hasOld := controller.limiter.UUIDtoUID["old"]
	require.False(t, hasOld)
	uid, hasNew := controller.limiter.UUIDtoUID["new"]
	require.True(t, hasNew)
	require.Equal(t, 2, uid)
}

func TestUserListMonitor_UnchangedNoop(t *testing.T) {
	initial := []domain.UserInfo{{Id: 1, Uuid: "old"}}
	userClient := &fakeUserListClient{response: nil}
	controller, server := newTestController(t, initial, userClient)

	err := controller.userListMonitor()
	require.NoError(t, err)
	require.Equal(t, 1, userClient.calls)
	require.Len(t, server.deleted, 0)
	require.Len(t, server.added, 0)
	require.Equal(t, initial, controller.userList)
	_, has := controller.limiter.UUIDtoUID["old"]
	require.True(t, has)
}
