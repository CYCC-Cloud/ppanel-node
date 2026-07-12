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
	require.Equal(t, "127.0.0.1:50051", c.ApiConfig.GRPCAddr)
	require.Equal(t, "node-secret", c.ApiConfig.GRPCSecret)
	require.True(t, c.ApiConfig.GRPCWatchControl)
}

func TestNew_GRPCWatchControlDefaultsTrue(t *testing.T) {
	c := conf.New()
	require.True(t, c.ApiConfig.GRPCWatchControl)
}

func TestLoadFromPath_GRPCWatchControlCanBeDisabled(t *testing.T) {
	c := conf.New()
	err := c.LoadFromPath("testdata/grpc_watch_disabled.yml")
	require.NoError(t, err)
	require.False(t, c.ApiConfig.GRPCWatchControl)
}
