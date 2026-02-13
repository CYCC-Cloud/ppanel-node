package grpcclient

import (
	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/api/panel"
)

// AdaptServerUsers converts gRPC server user metadata into panel.UserInfo.
func AdaptServerUsers(users []*nodecontrolv1.ServerUser) []panel.UserInfo {
	if len(users) == 0 {
		return []panel.UserInfo{}
	}
	result := make([]panel.UserInfo, 0, len(users))
	for _, u := range users {
		if u == nil {
			continue
		}
		result = append(result, panel.UserInfo{
			Id:          int(u.GetId()),
			Uuid:        u.GetUuid(),
			SpeedLimit:  int(u.GetSpeedLimit()),
			DeviceLimit: int(u.GetDeviceLimit()),
		})
	}
	return result
}
