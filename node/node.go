package node

import (
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
	for i := range n.controllers {
		err := n.controllers[i].Start()
		if err != nil {
			return fmt.Errorf("启动节点 [%s-%s-%d] 失败: %s",
				n.controllers[i].apiHost,
				n.controllers[i].info.Type,
				n.controllers[i].info.Id,
				err)
		}
	}
	return nil
}

func (n *Node) Close() {
	for _, c := range n.controllers {
		err := c.Close()
		if err != nil {
			panic(err)
		}
	}
	n.controllers = nil
}
