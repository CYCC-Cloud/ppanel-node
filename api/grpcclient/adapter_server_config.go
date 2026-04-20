package grpcclient

import (
	nodecontrolv1 "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/domain"
)

func AdaptServerConfigResponse(resp *nodecontrolv1.GetConfigResponse) *domain.ServerConfigResponse {
	if resp == nil || resp.GetData() == nil {
		return nil
	}

	data := resp.GetData()
	result := &domain.ServerConfigResponse{
		Code: 0,
		Msg:  "ok",
		Data: &domain.Data{
			TrafficReportThreshold: int(data.GetTrafficReportThreshold()),
			PushInterval:           int(data.GetPushIntervalSec()),
			PullInterval:           int(data.GetPullIntervalSec()),
			IPStrategy:             data.GetIpStrategy(),
			Total:                  int(data.GetTotal()),
		},
	}

	if len(data.GetDns()) > 0 {
		dns := make([]domain.DNSItem, 0, len(data.GetDns()))
		for _, item := range data.GetDns() {
			dns = append(dns, domain.DNSItem{
				Proto:   item.GetProto(),
				Address: item.GetAddress(),
				Domains: item.GetDomains(),
			})
		}
		result.Data.DNS = &dns
	}

	if len(data.GetBlock()) > 0 {
		block := append([]string{}, data.GetBlock()...)
		result.Data.Block = &block
	}

	if len(data.GetOutbound()) > 0 {
		outbound := make([]domain.Outbound, 0, len(data.GetOutbound()))
		for _, item := range data.GetOutbound() {
			outbound = append(outbound, domain.Outbound{
				Name:     item.GetName(),
				Protocol: item.GetProtocol(),
				Address:  item.GetAddress(),
				Port:     int(item.GetPort()),
				Password: item.GetPassword(),
				Rules:    item.GetRules(),
			})
		}
		result.Data.Outbound = &outbound
	}

	if len(data.GetProtocols()) > 0 {
		protocols := make([]domain.Protocol, 0, len(data.GetProtocols()))
		for _, item := range data.GetProtocols() {
			protocols = append(protocols, domain.Protocol{
				ListenerKey:             item.GetListenerKey(),
				ListenerName:            item.GetListenerName(),
				Type:                    item.GetType(),
				Port:                    int(item.GetPort()),
				Enable:                  item.GetEnable(),
				Security:                item.GetSecurity(),
				SNI:                     item.GetSni(),
				AllowInsecure:           item.GetAllowInsecure(),
				Fingerprint:             item.GetFingerprint(),
				RealityServerAddr:       item.GetRealityServerAddr(),
				RealityServerPort:       int(item.GetRealityServerPort()),
				RealityPrivateKey:       item.GetRealityPrivateKey(),
				RealityPublicKey:        item.GetRealityPublicKey(),
				RealityShortID:          item.GetRealityShortId(),
				Transport:               item.GetTransport(),
				Host:                    item.GetHost(),
				Path:                    item.GetPath(),
				ServiceName:             item.GetServiceName(),
				Cipher:                  item.GetCipher(),
				ServerKey:               item.GetServerKey(),
				Flow:                    item.GetFlow(),
				HopPorts:                item.GetHopPorts(),
				HopInterval:             int(item.GetHopInterval()),
				ObfsPassword:            item.GetObfsPassword(),
				DisableSNI:              item.GetDisableSni(),
				ReduceRTT:               item.GetReduceRtt(),
				UDPRelayMode:            item.GetUdpRelayMode(),
				CongestionController:    item.GetCongestionController(),
				Multiplex:               item.GetMultiplex(),
				PaddingScheme:           item.GetPaddingScheme(),
				UpMbps:                  int(item.GetUpMbps()),
				DownMbps:                int(item.GetDownMbps()),
				Obfs:                    item.GetObfs(),
				ObfsHost:                item.GetObfsHost(),
				ObfsPath:                item.GetObfsPath(),
				XHTTPMode:               item.GetXhttpMode(),
				XHTTPExtra:              item.GetXhttpExtra(),
				Encryption:              item.GetEncryption(),
				EncryptionMode:          item.GetEncryptionMode(),
				EncryptionRTT:           item.GetEncryptionRtt(),
				EncryptionTicket:        item.GetEncryptionTicket(),
				EncryptionServerPadding: item.GetEncryptionServerPadding(),
				EncryptionPrivateKey:    item.GetEncryptionPrivateKey(),
				EncryptionClientPadding: item.GetEncryptionClientPadding(),
				EncryptionPassword:      item.GetEncryptionPassword(),
				CertMode:                item.GetCertMode(),
				CertDNSProvider:         item.GetCertDnsProvider(),
				CertDNSEnv:              item.GetCertDnsEnv(),
			})
		}
		result.Data.Protocols = &protocols
	}

	return result
}
