package core

import (
	"context"
	"testing"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/stretchr/testify/require"
)

type mockServerConfigClient struct {
	resp            *nodecontrolv1.GetConfigResponse
	err             error
	knownRevisionIn string
	protocolsIn     []string
}

func (m *mockServerConfigClient) GetConfig(_ context.Context, knownRevision string, protocols []string) (*nodecontrolv1.GetConfigResponse, error) {
	m.knownRevisionIn = knownRevision
	m.protocolsIn = append([]string{}, protocols...)
	return m.resp, m.err
}

func (m *mockServerConfigClient) Close() error {
	return nil
}

func TestServerConfigMonitor_ChangedTriggersReload(t *testing.T) {
	reloadCh := make(chan struct{}, 1)
	client := &mockServerConfigClient{
		resp: &nodecontrolv1.GetConfigResponse{
			Changed:  true,
			Revision: "rev-2",
			Data:     &nodecontrolv1.ServerConfig{},
		},
	}

	x := &XrayCore{
		ConfigClient:         client,
		ReloadCh:             reloadCh,
		knownConfigRevision:  "rev-1",
		monitorProtocolTypes: []string{"vless"},
	}

	err := x.ServerConfigMonitor()
	require.NoError(t, err)
	require.Equal(t, "rev-1", client.knownRevisionIn)
	require.Equal(t, []string{"vless"}, client.protocolsIn)
	require.Equal(t, "rev-2", x.knownConfigRevision)

	select {
	case <-reloadCh:
	default:
		t.Fatal("expected reload signal")
	}
}

func TestServerConfigMonitor_UnchangedNoReload(t *testing.T) {
	reloadCh := make(chan struct{}, 1)
	client := &mockServerConfigClient{resp: nil}

	x := &XrayCore{
		ConfigClient:         client,
		ReloadCh:             reloadCh,
		knownConfigRevision:  "rev-1",
		monitorProtocolTypes: []string{"vless"},
	}

	err := x.ServerConfigMonitor()
	require.NoError(t, err)
	require.Equal(t, "rev-1", x.knownConfigRevision)

	select {
	case <-reloadCh:
		t.Fatal("unexpected reload signal")
	default:
	}
}
