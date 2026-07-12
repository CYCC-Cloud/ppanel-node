package node

import (
	"errors"
	"reflect"
	"testing"

	vCore "github.com/perfect-panel/ppanel-node/core"
	"github.com/perfect-panel/ppanel-node/domain"
	"github.com/perfect-panel/ppanel-node/limiter"
)

type lifecycleCoreServer struct {
	addCalls int
	delTags  []string
}

func (f *lifecycleCoreServer) AddNode(_ string, _ *domain.NodeInfo) error {
	f.addCalls++
	if f.addCalls == 2 {
		return errors.New("injected add failure")
	}
	return nil
}

func (f *lifecycleCoreServer) DelNode(tag string) error {
	f.delTags = append(f.delTags, tag)
	return nil
}

func (f *lifecycleCoreServer) AddUsers(_ *vCore.AddUsersParams) (int, error) {
	return 0, nil
}

func (f *lifecycleCoreServer) DelUsers(_ []domain.UserInfo, _ string, _ *domain.NodeInfo) error {
	return nil
}

func (f *lifecycleCoreServer) GetUserTrafficSlice(_ string, _ int) ([]domain.UserTraffic, error) {
	return nil, nil
}

func TestNodeStartRollsBackOnlyInitializedControllers(t *testing.T) {
	limiter.Init()
	server := &lifecycleCoreServer{}
	first := NewController(server, "pulse", nil, nil, lifecycleNodeInfo("first"))
	second := NewController(server, "pulse", nil, nil, lifecycleNodeInfo("second"))
	n := &Node{controllers: []*Controller{first, second}}

	if err := n.Start(); err == nil {
		t.Fatal("Start() error = nil, want injected failure")
	}

	wantDeleted := []string{"[pulse]-first"}
	if !reflect.DeepEqual(server.delTags, wantDeleted) {
		t.Fatalf("deleted tags = %v, want %v", server.delTags, wantDeleted)
	}
	if first.started || first.nodeAdded || first.limiter != nil {
		t.Fatal("first controller retained resources after rollback")
	}
	if second.started || second.nodeAdded || second.limiter != nil {
		t.Fatal("failing controller retained partial resources after rollback")
	}
	if err := n.Close(); err != nil {
		t.Fatalf("Close() after rollback error = %v", err)
	}
}

func lifecycleNodeInfo(listenerKey string) *domain.NodeInfo {
	return &domain.NodeInfo{
		Type:         "shadowsocks",
		PushInterval: 60,
		PullInterval: 60,
		Protocol: &domain.Protocol{
			ListenerKey: listenerKey,
			Type:        "shadowsocks",
		},
	}
}
