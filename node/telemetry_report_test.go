package node

import (
	"context"
	"fmt"
	"sync"
	"testing"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/api/panel"
	vCore "github.com/perfect-panel/ppanel-node/core"
	"github.com/perfect-panel/ppanel-node/limiter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTelemetryClient records sent batches for assertion.
type fakeTelemetryClient struct {
	sent []*nodecontrolv1.TelemetryBatch
	err  error
}

func (f *fakeTelemetryClient) ReportTelemetry(_ context.Context, batch *nodecontrolv1.TelemetryBatch) (*nodecontrolv1.ReportTelemetrySummary, error) {
	f.sent = append(f.sent, batch)
	return &nodecontrolv1.ReportTelemetrySummary{}, f.err
}

// fakeCoreServerForTelemetry provides coreServer with controllable traffic.
type fakeCoreServerForTelemetry struct {
	traffic []panel.UserTraffic
}

func (f *fakeCoreServerForTelemetry) AddNode(_ string, _ *panel.NodeInfo) error { return nil }
func (f *fakeCoreServerForTelemetry) DelNode(_ string) error                    { return nil }
func (f *fakeCoreServerForTelemetry) AddUsers(_ *vCore.AddUsersParams) (int, error) {
	return 0, nil
}
func (f *fakeCoreServerForTelemetry) DelUsers(_ []panel.UserInfo, _ string, _ *panel.NodeInfo) error {
	return nil
}
func (f *fakeCoreServerForTelemetry) GetUserTrafficSlice(_ string, _ int) ([]panel.UserTraffic, error) {
	return f.traffic, nil
}

func newControllerForTelemetryTest(srv coreServer, telClient telemetryReportClient) *Controller {
	nodeInfo := &panel.NodeInfo{
		Id:   42,
		Type: "vmess",
		Protocol: &panel.Protocol{
			Type: "vmess",
		},
	}
	return &Controller{
		server:          srv,
		telemetryClient: telClient,
		info:            nodeInfo,
		tag:             "[test]:vmess:42",
	}
}

// TestReportUserTrafficTask_BuildsTelemetryBatch asserts that traffic records
// are assembled into a TelemetryBatch and sent via ReportTelemetry.
func TestReportUserTrafficTask_BuildsTelemetryBatch(t *testing.T) {
	traffic := []panel.UserTraffic{
		{UID: 1, Upload: 1000, Download: 2000},
		{UID: 2, Upload: 500, Download: 1500},
	}
	srv := &fakeCoreServerForTelemetry{traffic: traffic}
	fake := &fakeTelemetryClient{}
	c := newControllerForTelemetryTest(srv, fake)

	err := c.reportUserTrafficTask()
	require.NoError(t, err)

	require.Len(t, fake.sent, 1, "expected exactly one TelemetryBatch")
	batch := fake.sent[0]
	assert.Equal(t, int64(42), batch.GetServerId())
	assert.Equal(t, "vmess", batch.GetProtocol())
	require.Len(t, batch.GetTraffic(), 2)
	assert.Equal(t, int64(1), batch.GetTraffic()[0].GetUid())
	assert.Equal(t, int64(1000), batch.GetTraffic()[0].GetUpload())
	assert.Equal(t, int64(2000), batch.GetTraffic()[0].GetDownload())
	assert.Greater(t, batch.GetWindowEndUnixMs(), int64(0))
}

// TestReportUserTrafficTask_OnlineUserFilterPreserved verifies that users with
// zero traffic are excluded from the online_users section of the batch.
func TestReportUserTrafficTask_OnlineUserFilterPreserved(t *testing.T) {
	// User 1 has traffic; user 2 has zero traffic → user 2 must be excluded from online list.
	traffic := []panel.UserTraffic{
		{UID: 1, Upload: 100, Download: 200},
		{UID: 2, Upload: 0, Download: 0},
	}
	srv := &fakeCoreServerForTelemetry{traffic: traffic}
	fake := &fakeTelemetryClient{}
	c := newControllerForTelemetryTest(srv, fake)

	// Seed limiter so both users appear as online.
	limiter.Init()
	lim := limiter.AddLimiter(c.tag, []panel.UserInfo{
		{Id: 1, Uuid: "uuid-1"},
		{Id: 2, Uuid: "uuid-2"},
	}, nil)
	for _, pair := range []struct {
		uuid string
		uid  int
		ip   string
	}{
		{"uuid-1", 1, "1.2.3.4"},
		{"uuid-2", 2, "5.6.7.8"},
	} {
		ipMap := new(sync.Map)
		ipMap.Store(pair.ip, pair.uid)
		lim.UserOnlineIP.Store(fmt.Sprintf("%s|%s", c.tag, pair.uuid), ipMap)
	}
	c.limiter = lim
	defer limiter.DeleteLimiter(c.tag)

	err := c.reportUserTrafficTask()
	require.NoError(t, err)

	require.Len(t, fake.sent, 1)
	batch := fake.sent[0]

	// Only user 1 (has traffic) should be in online_users.
	for _, ou := range batch.GetOnlineUsers() {
		assert.NotEqual(t, int64(2), ou.GetUid(),
			"user 2 has zero traffic and must be filtered from online list")
	}
}
