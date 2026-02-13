package domain_test

import (
	"testing"

	"github.com/perfect-panel/ppanel-node/domain"
)

func TestDomainTypesExist(t *testing.T) {
	_ = domain.NodeInfo{}
	_ = domain.UserInfo{}
	_ = domain.UserTraffic{}
	_ = domain.OnlineUser{}
	_ = domain.ServerConfigResponse{}
	_ = domain.Protocol{}
}
