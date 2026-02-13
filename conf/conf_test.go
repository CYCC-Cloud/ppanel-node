package conf_test

import (
	"testing"

	"github.com/perfect-panel/ppanel-node/conf"
	"github.com/stretchr/testify/require"
)

func TestLoadFromPath_WithGRPCFields(t *testing.T) {
	c := conf.New()
	err := c.LoadFromPath("testdata/grpc_config.yml")
	require.NoError(t, err)
	require.Equal(t, "grpc", c.ApiConfig.Transport)
	require.Equal(t, "127.0.0.1:50051", c.ApiConfig.GRPCAddr)
	require.Equal(t, "node-secret", c.ApiConfig.GRPCSecret)
}
