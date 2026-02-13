package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/api/grpcclient"
	"github.com/perfect-panel/ppanel-node/api/panel"
	"github.com/perfect-panel/ppanel-node/conf"
	"github.com/perfect-panel/ppanel-node/core"
	"github.com/perfect-panel/ppanel-node/domain"
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
	Run:   serverHandle,
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

func serverHandle(_ *cobra.Command, _ []string) {
	showVersion()
	c := conf.New()
	err := c.LoadFromPath(config)
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: true,
		DisableQuote:     true,
		PadLevelText:     false,
	})
	if err != nil {
		log.WithField("err", err).Error("读取配置文件失败")
		return
	}
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
	if c.LogConfig.Output != "" {
		f, err := os.OpenFile(c.LogConfig.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.WithField("err", err).Error("打开日志文件失败，使用stdout替代")
		}
		log.SetOutput(f)
	}
	// Enable pprof if configured
	if c.PprofPort != 0 {
		go func() {
			log.Infof("Starting pprof server on :%d", c.PprofPort)
			if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", c.PprofPort), nil); err != nil {
				log.WithField("err", err).Error("pprof server failed")
			}
		}()
	}
	limiter.Init()
	serverClient, serverconfig, revision, err := loadServerConfigClient(&c.ApiConfig)
	if err != nil {
		log.WithField("err", err).Error("获取服务端配置失败")
		return
	}
	if serverconfig == nil || serverconfig.Data == nil {
		log.Error("服务端配置为空")
		return
	}
	var reloadCh = make(chan struct{}, 1)
	xraycore := core.New(c, serverClient)
	xraycore.ReloadCh = reloadCh
	xraycore.SetKnownConfigRevision(revision)
	err = xraycore.Start(serverconfig)
	if err != nil {
		_ = xraycore.Close()
		log.WithField("err", err).Error("启动Xray核心失败")
		return
	}
	defer xraycore.Close()
	nodes, err := node.New(xraycore, c, serverconfig)
	if err != nil {
		log.WithField("err", err).Error("获取节点配置失败")
		return
	}
	err = nodes.Start()
	if err != nil {
		log.WithField("err", err).Error("启动节点失败")
		return
	}
	log.Infof("已启动 %d 个节点", serverconfig.Data.Total)
	if watch {
		// On file change, just signal reload; do not run reload concurrently here
		err = c.Watch(config, func() {
			select {
			case reloadCh <- struct{}{}:
			default: // drop if a reload is already queued
			}
		})
		if err != nil {
			log.WithField("err", err).Error("start watch failed")
			return
		}
	}
	// clear memory
	runtime.GC()

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-osSignals:
			nodes.Close()
			_ = xraycore.Close()
			return
		case <-reloadCh:
			log.Info("收到重启信号，正在重新加载配置...")
			if err := reload(config, &nodes, &xraycore); err != nil {
				log.WithField("err", err).Error("重启失败")
			}
		}
	}
}

func reload(config string, nodes **node.Node, xcore **core.XrayCore) error {
	// Preserve old reload channel so new core continues to receive signals
	var oldReloadCh chan struct{}

	if *xcore != nil {
		oldReloadCh = (*xcore).ReloadCh
	}

	(*nodes).Close()
	if err := (*xcore).Close(); err != nil {
		return err
	}

	newConf := conf.New()
	if err := newConf.LoadFromPath(config); err != nil {
		return err
	}

	serverClient, serverconfig, revision, err := loadServerConfigClient(&newConf.ApiConfig)
	if err != nil {
		log.WithField("err", err).Error("获取服务端配置失败")
		return err
	}
	if serverconfig == nil || serverconfig.Data == nil {
		return fmt.Errorf("服务端配置为空")
	}

	newCore := core.New(newConf, serverClient)
	// Reattach reload channel
	newCore.ReloadCh = oldReloadCh
	newCore.SetKnownConfigRevision(revision)
	if err := newCore.Start(serverconfig); err != nil {
		_ = newCore.Close()
		return err
	}
	newNodes, err := node.New(newCore, newConf, serverconfig)
	if err != nil {
		_ = newCore.Close()
		return err
	}
	if err := newNodes.Start(); err != nil {
		_ = newCore.Close()
		return err
	}

	*nodes = newNodes
	*xcore = newCore
	log.Infof("%d 个节点重启成功", serverconfig.Data.Total)
	runtime.GC()
	return nil
}

type httpServerConfigClient struct {
	client *panel.ClientV2
}

func (h *httpServerConfigClient) GetConfig(_ context.Context, _ string, _ []string) (*nodecontrolv1.GetConfigResponse, error) {
	newServerConfig, err := panel.GetServerConfig(h.client)
	if err != nil {
		return nil, err
	}
	if newServerConfig == nil {
		return nil, nil
	}
	return &nodecontrolv1.GetConfigResponse{Changed: true}, nil
}

func (h *httpServerConfigClient) Close() error {
	return nil
}

func loadServerConfigClient(apiConfig *conf.ServerApiConfig) (core.ServerConfigClient, *domain.ServerConfigResponse, string, error) {
	if strings.EqualFold(apiConfig.Transport, "http") {
		httpClient := panel.NewClientV2(apiConfig)
		panelConfig, err := panel.GetServerConfig(httpClient)
		if err != nil {
			return nil, nil, "", err
		}
		domainConfig, err := panelServerConfigToDomain(panelConfig)
		if err != nil {
			return nil, nil, "", err
		}
		return &httpServerConfigClient{client: httpClient}, domainConfig, "", nil
	}

	grpcClient, err := grpcclient.New(apiConfig)
	if err != nil {
		return nil, nil, "", err
	}

	resp, err := grpcClient.GetConfig(context.Background(), "", nil)
	if err != nil {
		_ = grpcClient.Close()
		return nil, nil, "", err
	}
	if resp == nil || resp.GetData() == nil {
		_ = grpcClient.Close()
		return nil, nil, "", fmt.Errorf("grpc 返回空配置")
	}

	serverconfig := grpcclient.AdaptServerConfigResponse(resp)
	if serverconfig == nil || serverconfig.Data == nil {
		_ = grpcClient.Close()
		return nil, nil, "", fmt.Errorf("grpc 配置转换失败")
	}

	return grpcClient, serverconfig, resp.GetRevision(), nil
}

func panelServerConfigToDomain(resp *panel.ServerConfigResponse) (*domain.ServerConfigResponse, error) {
	if resp == nil {
		return nil, nil
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var result domain.ServerConfigResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
