package node

import (
	"context"
	"fmt"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/api/grpcclient"
	"github.com/perfect-panel/ppanel-node/common/task"
	vCore "github.com/perfect-panel/ppanel-node/core"
	"github.com/perfect-panel/ppanel-node/domain"
	"github.com/perfect-panel/ppanel-node/limiter"
	log "github.com/sirupsen/logrus"
)

type coreServer interface {
	AddNode(tag string, info *domain.NodeInfo) error
	DelNode(tag string) error
	AddUsers(params *vCore.AddUsersParams) (int, error)
	DelUsers(users []domain.UserInfo, tag string, info *domain.NodeInfo) error
	GetUserTrafficSlice(tag string, mintraffic int) ([]domain.UserTraffic, error)
}

type userListClient interface {
	GetUserList(ctx context.Context, listenerKey, knownRevision string) (*nodecontrolv1.GetUserListResponse, error)
}

type telemetryReportClient interface {
	ReportTelemetry(ctx context.Context, batch *nodecontrolv1.TelemetryBatch) (*nodecontrolv1.ReportTelemetrySummary, error)
}

type Controller struct {
	server                  coreServer
	apiHost                 string
	tag                     string
	nodeAdded               bool
	started                 bool
	limiter                 *limiter.Limiter
	userList                []domain.UserInfo
	info                    *domain.NodeInfo
	userListMonitorPeriodic *task.Task
	userReportPeriodic      *task.Task
	renewCertPeriodic       *task.Task
	onlineIpReportPeriodic  *task.Task
	userClient              userListClient
	telemetryClient         telemetryReportClient
	knownRevision           string
}

// NewController return a Node controller with default parameters.
func NewController(server coreServer, apiHost string, userClient userListClient, telemetryClient telemetryReportClient, info *domain.NodeInfo) *Controller {
	return &Controller{
		server:          server,
		apiHost:         apiHost,
		userClient:      userClient,
		telemetryClient: telemetryClient,
		info:            info,
	}
}

// Start implement the Start() function of the service interface
func (c *Controller) Start() error {
	var err error
	// Update user
	c.userList, err = c.fetchUserList()
	if err != nil {
		return fmt.Errorf("get user list error: %s", err)
	}
	c.tag = c.buildNodeTag(c.info)

	// add limiter
	l := limiter.AddLimiter(c.tag, c.userList, nil)
	c.limiter = l

	if c.info.Protocol.Security == "tls" {
		err = c.requestCert()
		if err != nil {
			return fmt.Errorf("request cert error: %s", err)
		}
	}
	// Add new tag
	err = c.server.AddNode(c.tag, c.info)
	if err != nil {
		return fmt.Errorf("add new node error: %s", err)
	}
	c.nodeAdded = true
	added, err := c.server.AddUsers(&vCore.AddUsersParams{
		Tag:      c.tag,
		Users:    c.userList,
		NodeInfo: c.info,
	})
	if err != nil {
		return fmt.Errorf("add users error: %s", err)
	}
	log.WithField("节点", c.tag).Infof("已添加 %d 个新用户", added)
	c.startTasks(c.info)
	c.started = true
	return nil
}

// Close implement the Close() function of the service interface
func (c *Controller) Close() error {
	if c == nil {
		return nil
	}
	if c.userListMonitorPeriodic != nil {
		c.userListMonitorPeriodic.Close()
		c.userListMonitorPeriodic = nil
	}
	if c.userReportPeriodic != nil {
		c.userReportPeriodic.Close()
		c.userReportPeriodic = nil
	}
	if c.renewCertPeriodic != nil {
		c.renewCertPeriodic.Close()
		c.renewCertPeriodic = nil
	}
	if c.onlineIpReportPeriodic != nil {
		c.onlineIpReportPeriodic.Close()
		c.onlineIpReportPeriodic = nil
	}

	if c.started {
		log.WithField("node", c.tag).Info("Node closing; sending final telemetry report")
		if err := c.reportUserTrafficTask(); err != nil {
			log.WithField("node", c.tag).WithError(err).Warn("Final telemetry report failed")
		}
		c.started = false
	}
	if c.limiter != nil {
		limiter.DeleteLimiter(c.tag)
		c.limiter = nil
	}
	if !c.nodeAdded {
		return nil
	}
	c.nodeAdded = false
	if err := c.server.DelNode(c.tag); err != nil {
		return fmt.Errorf("delete node: %w", err)
	}
	return nil
}

func (c *Controller) buildNodeTag(node *domain.NodeInfo) string {
	return fmt.Sprintf("[%s]-%s", c.apiHost, node.Protocol.ListenerKey)
}

func (c *Controller) fetchUserList() ([]domain.UserInfo, error) {
	if c.userClient != nil && c.info != nil && c.info.Protocol != nil {
		resp, err := c.userClient.GetUserList(context.Background(), c.info.Protocol.ListenerKey, c.knownRevision)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, nil
		}
		c.knownRevision = resp.GetRevision()
		return grpcclient.AdaptServerUsers(resp.GetUsers()), nil
	}
	return nil, nil
}
