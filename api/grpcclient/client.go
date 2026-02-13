package grpcclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"time"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/conf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultDialTimeout = 5 * time.Second
	defaultRPCTimeout  = 10 * time.Second
)

type authMetadataCredentials struct {
	secret                   string
	nodeID                   string
	requireTransportSecurity bool
}

func (a *authMetadataCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"x-node-secret": a.secret,
		"x-node-id":     a.nodeID,
	}, nil
}

func (a *authMetadataCredentials) RequireTransportSecurity() bool {
	return a.requireTransportSecurity
}

type Client struct {
	conn        *grpc.ClientConn
	nodeControl nodecontrolv1.NodeControlServiceClient
	serverID    int64
	rpcTimeout  time.Duration
}

func New(cfg *conf.ServerApiConfig) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("grpc config is nil")
	}
	if cfg.GRPCAddr == "" {
		return nil, errors.New("grpc addr is empty")
	}

	dialTimeout := secondsToDuration(cfg.GRPCDialTimeout, defaultDialTimeout)
	rpcTimeout := secondsToDuration(cfg.GRPCRPCTimeout, defaultRPCTimeout)
	secret := cfg.GRPCSecret
	if secret == "" {
		secret = cfg.SecretKey
	}

	transportCreds := insecure.NewCredentials()
	if cfg.GRPCTLS {
		transportCreds = credentials.NewTLS(&tls.Config{
			ServerName:         cfg.GRPCServerName,
			InsecureSkipVerify: cfg.GRPCInsecureSkipVerify,
		})
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		cfg.GRPCAddr,
		grpc.WithTransportCredentials(transportCreds),
		grpc.WithPerRPCCredentials(&authMetadataCredentials{
			secret:                   secret,
			nodeID:                   strconv.Itoa(cfg.ServerId),
			requireTransportSecurity: cfg.GRPCTLS,
		}),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial grpc %s: %w", cfg.GRPCAddr, err)
	}

	return &Client{
		conn:        conn,
		nodeControl: nodecontrolv1.NewNodeControlServiceClient(conn),
		serverID:    int64(cfg.ServerId),
		rpcTimeout:  rpcTimeout,
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) rpcContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, c.rpcTimeout)
}

func secondsToDuration(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}
