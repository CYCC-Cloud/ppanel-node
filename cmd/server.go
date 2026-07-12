package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"sync"
	"syscall"

	pb "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/google/uuid"
	"github.com/perfect-panel/ppanel-node/api/grpcclient"
	"github.com/perfect-panel/ppanel-node/conf"
	"github.com/perfect-panel/ppanel-node/core"
	"github.com/perfect-panel/ppanel-node/limiter"
	"github.com/perfect-panel/ppanel-node/node"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	config string
	watch  bool
)

var serverCommand = cobra.Command{
	Use:   "server",
	Short: "Run ppnode server",
	RunE:  serverHandle,
	Args:  cobra.NoArgs,
}

func init() {
	serverCommand.PersistentFlags().
		StringVarP(&config, "config", "c",
			"/etc/PPanel-node/config.yml", "config file path")
	serverCommand.PersistentFlags().
		BoolVarP(&watch, "watch", "w",
			true, "watch file path change")
	command.AddCommand(&serverCommand)
}

type serverRuntime struct {
	client  *grpcclient.Client
	core    *core.XrayCore
	nodes   *node.Node
	watcher *controlWatcher

	closeOnce sync.Once
	closeErr  error
}

func (r *serverRuntime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.watcher != nil {
			r.watcher.Stop()
			r.watcher = nil
		}
		if r.nodes != nil {
			r.closeErr = errors.Join(r.closeErr, r.nodes.Close())
			r.nodes = nil
		}
		if r.core != nil {
			// The runtime owns the client and closes it after the core.
			r.core.ConfigClient = nil
			r.closeErr = errors.Join(r.closeErr, r.core.Close())
			r.core = nil
		}
		if r.client != nil {
			r.closeErr = errors.Join(r.closeErr, r.client.Close())
			r.client = nil
		}
	})
	return r.closeErr
}

var (
	prepareRuntimeForReload = prepareRuntime
	startRuntimeForReload   = startRuntime
)

func serverHandle(_ *cobra.Command, _ []string) error {
	showVersion()

	c, client, serverConfig, revision, err := prepareRuntime(config)
	if err != nil {
		return fmt.Errorf("prepare server runtime: %w", err)
	}

	configureLogging(c)
	if c.PprofPort != 0 {
		go func() {
			log.Infof("Starting pprof server on :%d", c.PprofPort)
			if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", c.PprofPort), nil); err != nil {
				log.WithError(err).Error("pprof server failed")
			}
		}()
	}

	limiter.Init()
	reloadCh := make(chan struct{}, 1)
	nodeInstanceID := uuid.NewString()
	current, err := startRuntime(c, client, serverConfig, revision, reloadCh, nodeInstanceID)
	if err != nil {
		return fmt.Errorf("start server runtime: %w", err)
	}
	defer func() {
		if err := current.Close(); err != nil {
			log.WithError(err).Error("Failed to close server runtime")
		}
	}()

	if watch {
		watchConf := conf.New()
		if err := watchConf.Watch(config, func() {
			select {
			case reloadCh <- struct{}{}:
			default:
			}
		}); err != nil {
			return fmt.Errorf("start config file watcher: %w", err)
		}
	}

	runtime.GC()
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(osSignals)

	for {
		select {
		case <-osSignals:
			return nil
		case <-reloadCh:
			log.Info("Reloading server configuration")
			if err := reload(config, &current, reloadCh, nodeInstanceID); err != nil {
				if current == nil {
					return fmt.Errorf("reload server runtime after cutover: %w", err)
				}
				log.WithError(err).Error("Server configuration preflight failed; keeping current runtime")
			}
		}
	}
}

func configureLogging(c *conf.Conf) {
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: true,
		DisableQuote:     true,
		PadLevelText:     false,
	})
	switch c.LogConfig.Level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn", "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	}
	if c.LogConfig.Output == "" {
		return
	}
	f, err := os.OpenFile(c.LogConfig.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.WithError(err).Error("Failed to open log file; continuing with stdout")
		return
	}
	log.SetOutput(f)
}

func prepareRuntime(configPath string) (*conf.Conf, *grpcclient.Client, *pb.GetConfigResponse, string, error) {
	c := conf.New()
	if err := c.LoadFromPath(configPath); err != nil {
		return nil, nil, nil, "", err
	}
	client, serverConfig, revision, err := loadServerConfigClient(&c.ApiConfig)
	if err != nil {
		return nil, nil, nil, "", err
	}
	return c, client, serverConfig, revision, nil
}

func startRuntime(c *conf.Conf, client *grpcclient.Client, serverConfig *pb.GetConfigResponse, revision string, reloadCh chan struct{}, nodeInstanceID string) (*serverRuntime, error) {
	r := &serverRuntime{client: client}
	if c == nil || client == nil || serverConfig == nil || serverConfig.GetData() == nil {
		_ = r.Close()
		return nil, errors.New("server runtime input is incomplete")
	}

	adapted := grpcclient.AdaptServerConfigResponse(serverConfig)
	if adapted == nil || adapted.Data == nil {
		_ = r.Close()
		return nil, errors.New("convert gRPC server configuration")
	}

	r.core = core.New(c, client)
	r.core.ReloadCh = reloadCh
	r.core.SetKnownConfigRevision(revision)
	if err := r.core.Start(adapted); err != nil {
		_ = r.Close()
		return nil, fmt.Errorf("start Xray core: %w", err)
	}

	var err error
	r.nodes, err = node.New(r.core, c, adapted)
	if err != nil {
		_ = r.Close()
		return nil, fmt.Errorf("create nodes: %w", err)
	}
	if err := r.nodes.Start(); err != nil {
		_ = r.Close()
		return nil, fmt.Errorf("start nodes: %w", err)
	}

	if c.ApiConfig.GRPCWatchControl {
		r.watcher = startControlWatcher(client, watchControlRequest(c, serverConfig, nodeInstanceID), reloadCh)
	}
	log.Infof("Started %d nodes", adapted.Data.Total)
	return r, nil
}

func reload(configPath string, current **serverRuntime, reloadCh chan struct{}, nodeInstanceID string) error {
	c, client, serverConfig, revision, err := prepareRuntimeForReload(configPath)
	if err != nil {
		return err
	}

	old := *current
	if old != nil {
		if err := old.Close(); err != nil {
			*current = nil
			_ = client.Close()
			return fmt.Errorf("close current runtime: %w", err)
		}
	}
	*current = nil

	newRuntime, err := startRuntimeForReload(c, client, serverConfig, revision, reloadCh, nodeInstanceID)
	if err != nil {
		return err
	}
	*current = newRuntime
	log.Info("Server configuration reload completed")
	runtime.GC()
	return nil
}

func loadServerConfigClient(apiConfig *conf.ServerApiConfig) (*grpcclient.Client, *pb.GetConfigResponse, string, error) {
	client, err := grpcclient.New(apiConfig)
	if err != nil {
		return nil, nil, "", err
	}
	resp, err := client.GetConfig(context.Background(), "", nil)
	if err != nil {
		_ = client.Close()
		return nil, nil, "", err
	}
	if resp == nil || resp.GetData() == nil {
		_ = client.Close()
		return nil, nil, "", errors.New("gRPC returned an empty server configuration")
	}
	return client, resp, resp.GetRevision(), nil
}

func watchControlRequest(c *conf.Conf, serverConfig *pb.GetConfigResponse, nodeInstanceID string) *pb.WatchControlRequest {
	return &pb.WatchControlRequest{
		ServerId:       int64(c.ApiConfig.ServerId),
		Protocols:      protocolTypes(serverConfig),
		NodeInstanceId: nodeInstanceID,
		NodeVersion:    version,
	}
}

func protocolTypes(serverConfig *pb.GetConfigResponse) []string {
	if serverConfig == nil {
		return nil
	}
	data := serverConfig.GetData()
	if data == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(data.GetProtocols()))
	protocols := make([]string, 0, len(data.GetProtocols()))
	for _, protocol := range data.GetProtocols() {
		protocolType := protocol.GetType()
		if protocolType == "" {
			continue
		}
		if _, ok := seen[protocolType]; ok {
			continue
		}
		seen[protocolType] = struct{}{}
		protocols = append(protocols, protocolType)
	}
	sort.Strings(protocols)
	return protocols
}
