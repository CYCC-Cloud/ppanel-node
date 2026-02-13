package grpcclient

import (
	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/domain"
)

// AdaptServerUsers converts gRPC server user metadata into domain.UserInfo.
func AdaptServerUsers(users []*nodecontrolv1.ServerUser) []domain.UserInfo {
	if len(users) == 0 {
		return []domain.UserInfo{}
	}
	result := make([]domain.UserInfo, 0, len(users))
	for _, u := range users {
		if u == nil {
			continue
		}
		result = append(result, domain.UserInfo{
			Id:          int(u.GetId()),
			Uuid:        u.GetUuid(),
			SpeedLimit:  int(u.GetSpeedLimit()),
			DeviceLimit: int(u.GetDeviceLimit()),
		})
	}
	return result
}
