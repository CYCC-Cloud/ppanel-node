package node

import (
	"errors"
	"fmt"

	"github.com/perfect-panel/ppanel-node/conf"
	vCore "github.com/perfect-panel/ppanel-node/core"
	"github.com/perfect-panel/ppanel-node/domain"
)

type Node struct {
	controllers []*Controller
}

func New(core *vCore.XrayCore, config *conf.Conf, serverconfig *domain.ServerConfigResponse) (*Node, error) {
	node := &Node{
		controllers: make([]*Controller, len(*serverconfig.Data.Protocols)),
	}
	pushinterval := serverconfig.Data.PushInterval
	if pushinterval <= 0 {
		pushinterval = 60
	}
	pullinterval := serverconfig.Data.PullInterval
	if pullinterval <= 0 {
		pullinterval = 60
	}
	var userClient userListClient
	var telClient telemetryReportClient
	if core != nil && core.ConfigClient != nil {
		if cli, ok := core.ConfigClient.(userListClient); ok {
			userClient = cli
		}
		if cli, ok := core.ConfigClient.(telemetryReportClient); ok {
			telClient = cli
		}
	}
	apiHost := config.ApiConfig.GRPCAddr

	for i, nodeconfig := range *serverconfig.Data.Protocols {
		n := &domain.NodeInfo{
			Id:                     config.ApiConfig.ServerId,
			Type:                   nodeconfig.Type,
			TrafficReportThreshold: serverconfig.Data.TrafficReportThreshold,
			PushInterval:           pushinterval,
			PullInterval:           pullinterval,
			Protocol:               &nodeconfig,
		}
		node.controllers[i] = NewController(core, apiHost, userClient, telClient, n)
	}

	return node, nil
}

func (n *Node) Start() error {
	for i, controller := range n.controllers {
		if err := controller.Start(); err != nil {
			rollbackErr := closeControllers(n.controllers[:i+1])
			return errors.Join(
				fmt.Errorf("start node [%s-%s]: %w",
					controller.apiHost,
					controller.info.Protocol.ListenerKey,
					err,
				),
				rollbackErr,
			)
		}
	}
	return nil
}

func (n *Node) Close() error {
	if n == nil {
		return nil
	}
	err := closeControllers(n.controllers)
	n.controllers = nil
	return err
}

func closeControllers(controllers []*Controller) error {
	var err error
	for i := len(controllers) - 1; i >= 0; i-- {
		if controllers[i] != nil {
			err = errors.Join(err, controllers[i].Close())
		}
	}
	return err
}
