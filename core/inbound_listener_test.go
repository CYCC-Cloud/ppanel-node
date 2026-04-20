package core

import (
	"path/filepath"
	"testing"

	"github.com/perfect-panel/ppanel-node/domain"
	"github.com/stretchr/testify/require"
)

func TestTLSCertFilePaths_UseListenerKey(t *testing.T) {
	nodeInfo := &domain.NodeInfo{
		Id:   42,
		Type: "vless",
		Protocol: &domain.Protocol{
			ListenerKey: "listener-edge-1",
		},
	}

	certFile, keyFile := tlsCertFilePaths(nodeInfo)
	require.Equal(t, filepath.Join("/etc/PPanel-node/", "listener-edge-1.cer"), certFile)
	require.Equal(t, filepath.Join("/etc/PPanel-node/", "listener-edge-1.key"), keyFile)
}
