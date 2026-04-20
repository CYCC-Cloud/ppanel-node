package node

import (
	"testing"

	"github.com/perfect-panel/ppanel-node/domain"
	"github.com/stretchr/testify/require"
)

func TestBuildNodeTag_UsesListenerKey(t *testing.T) {
	controller := &Controller{apiHost: "grpc.example.com"}
	nodeInfo := &domain.NodeInfo{
		Protocol: &domain.Protocol{
			ListenerKey: "listener-edge-1",
			Type:        "vless",
		},
	}

	require.Equal(t, "[grpc.example.com]-listener-edge-1", controller.buildNodeTag(nodeInfo))
}
