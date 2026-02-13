package node

import (
	"context"
	"strconv"
	"time"

	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/common/serverstatus"
	"github.com/perfect-panel/ppanel-node/common/task"
	vCore "github.com/perfect-panel/ppanel-node/core"
	"github.com/perfect-panel/ppanel-node/domain"
	log "github.com/sirupsen/logrus"
)

func (c *Controller) startTasks(node *domain.NodeInfo) {
	// fetch user list task
	c.userListMonitorPeriodic = &task.Task{
		Name:     "userListMonitor",
		Interval: time.Duration(node.PullInterval) * time.Second,
		Execute:  c.userListMonitor,
		Reload:   c.reloadTask,
	}
	// report user traffic task
	c.userReportPeriodic = &task.Task{
		Name:     "reportUserTraffic",
		Interval: time.Duration(node.PushInterval) * time.Second,
		Execute:  c.reportUserTrafficTask,
		Reload:   c.reloadTask,
	}
	_ = c.userListMonitorPeriodic.Start(false)
	log.WithField("节点", c.tag).Info("用户列表监控任务已启动")
	_ = c.userReportPeriodic.Start(false)
	log.WithField("节点", c.tag).Info("用户流量报告任务已启动")
	var security string
	switch node.Type {
	case "vless":
		security = node.Protocol.Security
	case "vmess":
		security = node.Protocol.Security
	case "trojan":
		security = node.Protocol.Security
	case "shadowsocks":
		security = ""
	case "tuic":
		security = "tls"
	case "hysteria", "hysteria2":
		security = "tls"
	default:
		security = ""
	}

	if security == "tls" {
		switch node.Protocol.CertMode {
		case "none", "", "file", "self":
		default:
			c.renewCertPeriodic = &task.Task{
				Name:     "renewCert",
				Interval: time.Hour * 24,
				Execute:  c.renewCertTask,
				Reload:   c.reloadTask,
			}
			log.WithField("节点", c.tag).Info("证书定期更新任务已启动")
			// delay to start renewCert
			_ = c.renewCertPeriodic.Start(true)
		}
	}
}

func (c *Controller) reloadTask() {
	c.userListMonitorPeriodic.Close()
	c.userReportPeriodic.Close()
	if c.renewCertPeriodic != nil {
		c.renewCertPeriodic.Close()
	}
	c.startTasks(c.info)
}

func (c *Controller) userListMonitor() (err error) {
	// get user info
	newU, err := c.fetchUserList()
	if err != nil {
		log.WithFields(log.Fields{
			"tag": c.tag,
			"err": err,
		}).Error("Get user list failed")
		return nil
	}
	// update user list
	// newU == nil indicates 304 Not Modified; empty slice means the list is empty
	if newU == nil {
		return nil
	}
	deleted, added := compareUserList(c.userList, newU)
	if len(deleted) > 0 {
		// have deleted users
		err = c.server.DelUsers(deleted, c.tag, c.info)
		if err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
			}).Error("Delete users failed")
			return nil
		}
	}
	if len(added) > 0 {
		// have added users
		_, err = c.server.AddUsers(&vCore.AddUsersParams{
			Tag:      c.tag,
			NodeInfo: c.info,
			Users:    added,
		})
		if err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
			}).Error("Add users failed")
			return nil
		}
	}
	if len(added) > 0 || len(deleted) > 0 {
		// update Limiter
		c.limiter.UpdateUser(c.tag, added, deleted)
		if err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
			}).Error("limiter users failed")
			return nil
		}
	}
	c.userList = newU
	if len(added)+len(deleted) != 0 {
		log.WithField("节点", c.tag).
			Infof("删除 %d 个用户，新增 %d 个用户", len(deleted), len(added))
	}
	return nil
}

func (c *Controller) reportUserTrafficTask() error {
	var reportmin = 0
	if c.info.TrafficReportThreshold > 0 {
		reportmin = c.info.TrafficReportThreshold
	}

	userTraffic, _ := c.server.GetUserTrafficSlice(c.tag, reportmin)

	// Build proto traffic list
	protoTraffic := make([]*nodecontrolv1.UserTraffic, 0, len(userTraffic))
	for _, t := range userTraffic {
		protoTraffic = append(protoTraffic, &nodecontrolv1.UserTraffic{
			Uid:      int64(t.UID),
			Upload:   t.Upload,
			Download: t.Download,
		})
	}

	// Build online users — preserve rule: zero-traffic users excluded
	var protoOnline []*nodecontrolv1.OnlineUser
	if c.limiter != nil {
		if onlineDevice, err := c.limiter.GetOnlineDevice(); err == nil && len(*onlineDevice) > 0 {
			nocountUID := make(map[int]struct{})
			for _, t := range userTraffic {
				if t.Upload+t.Download <= 0 {
					nocountUID[t.UID] = struct{}{}
				}
			}
			for _, online := range *onlineDevice {
				if _, skip := nocountUID[online.UID]; !skip {
					protoOnline = append(protoOnline, &nodecontrolv1.OnlineUser{
						Uid: int64(online.UID),
						Ip:  online.IP,
					})
				}
			}
		}
	}

	// Build node status
	CPU, Mem, Disk, Uptime, err := serverstatus.GetSystemInfo()
	if err != nil {
		log.WithField("tag", c.tag).WithError(err).Warn("GetSystemInfo failed, reporting zero status")
	}

	protocol := ""
	if c.info != nil && c.info.Protocol != nil {
		protocol = c.info.Protocol.Type
	}

	now := time.Now().UnixMilli()
	batch := &nodecontrolv1.TelemetryBatch{
		ServerId:          int64(c.info.Id),
		Protocol:          protocol,
		WindowStartUnixMs: now - int64(c.info.PushInterval)*1000,
		WindowEndUnixMs:   now,
		Traffic:           protoTraffic,
		OnlineUsers:       protoOnline,
		Status: &nodecontrolv1.NodeStatus{
			Cpu:    CPU,
			Mem:    Mem,
			Disk:   Disk,
			Uptime: Uptime,
		},
	}

	if c.telemetryClient != nil {
		if _, err := c.telemetryClient.ReportTelemetry(context.Background(), batch); err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
			}).Warn("ReportTelemetry failed, will retry next cycle")
		} else {
			log.WithField("节点", c.tag).Infof(
				"已上报 telemetry: %d 流量, %d 在线用户",
				len(protoTraffic), len(protoOnline),
			)
		}
	}
	return nil
}

func compareUserList(old, new []domain.UserInfo) (deleted, added []domain.UserInfo) {
	oldMap := make(map[string]int)
	for i, user := range old {
		key := user.Uuid + strconv.Itoa(user.SpeedLimit)
		oldMap[key] = i
	}

	for _, user := range new {
		key := user.Uuid + strconv.Itoa(user.SpeedLimit)
		if _, exists := oldMap[key]; !exists {
			added = append(added, user)
		} else {
			delete(oldMap, key)
		}
	}

	for _, index := range oldMap {
		deleted = append(deleted, old[index])
	}

	return deleted, added
}
