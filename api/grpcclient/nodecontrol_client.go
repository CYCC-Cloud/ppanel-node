package grpcclient

import (
	"context"
	"errors"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
)

func (c *Client) GetConfig(ctx context.Context, knownRevision string, protocols []string) (*nodecontrolv1.GetConfigResponse, error) {
	if c == nil || c.nodeControl == nil {
		return nil, errors.New("grpc client is not initialized")
	}

	rpcCtx, cancel := c.rpcContext(ctx)
	defer cancel()

	resp, err := c.nodeControl.GetConfig(rpcCtx, &nodecontrolv1.GetConfigRequest{
		ServerId:      c.serverID,
		Protocols:     protocols,
		KnownRevision: knownRevision,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || !resp.GetChanged() {
		return nil, nil
	}
	return resp, nil
}

func (c *Client) GetUserList(ctx context.Context, protocol, knownRevision string) (*nodecontrolv1.GetUserListResponse, error) {
	if c == nil || c.nodeControl == nil {
		return nil, errors.New("grpc client is not initialized")
	}

	rpcCtx, cancel := c.rpcContext(ctx)
	defer cancel()

	resp, err := c.nodeControl.GetUserList(rpcCtx, &nodecontrolv1.GetUserListRequest{
		ServerId:      c.serverID,
		Protocol:      protocol,
		KnownRevision: knownRevision,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || !resp.GetChanged() {
		return nil, nil
	}
	return resp, nil
}

func (c *Client) ReportTelemetry(ctx context.Context, batch *nodecontrolv1.TelemetryBatch) (*nodecontrolv1.ReportTelemetrySummary, error) {
	if c == nil || c.nodeControl == nil {
		return nil, errors.New("grpc client is not initialized")
	}
	if batch == nil {
		return nil, errors.New("telemetry batch is nil")
	}

	rpcCtx, cancel := c.rpcContext(ctx)
	defer cancel()

	stream, err := c.nodeControl.ReportTelemetry(rpcCtx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(batch); err != nil {
		return nil, err
	}
	return stream.CloseAndRecv()
}
