package grpcclient

import (
	"testing"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/stretchr/testify/require"
)

func TestAdaptServerConfigResponse_MapsListenerFields(t *testing.T) {
	resp := &nodecontrolv1.GetConfigResponse{
		Data: &nodecontrolv1.ServerConfig{
			Protocols: []*nodecontrolv1.Protocol{{
				ListenerKey:  "listener-edge-1",
				ListenerName: "Edge Listener",
				Type:         "vless",
				Port:         443,
			}},
		},
	}

	adapted := AdaptServerConfigResponse(resp)
	require.NotNil(t, adapted)
	require.NotNil(t, adapted.Data)
	require.NotNil(t, adapted.Data.Protocols)
	require.Len(t, *adapted.Data.Protocols, 1)

	protocol := (*adapted.Data.Protocols)[0]
	require.Equal(t, "listener-edge-1", protocol.ListenerKey)
	require.Equal(t, "Edge Listener", protocol.ListenerName)
	require.Equal(t, "vless", protocol.Type)
	require.Equal(t, 443, protocol.Port)
}
