package domain

// ─── Server Config ────────────────────────────────────────────────────────────

type ServerConfigResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data *Data  `json:"data"`
}

type Data struct {
	TrafficReportThreshold int         `json:"traffic_report_threshold"`
	PushInterval           int         `json:"push_interval"`
	PullInterval           int         `json:"pull_interval"`
	IPStrategy             string      `json:"ip_strategy"`
	DNS                    *[]DNSItem  `json:"dns"`
	Block                  *[]string   `json:"block"`
	Outbound               *[]Outbound `json:"outbound"`
	Protocols              *[]Protocol `json:"protocols"`
	Total                  int         `json:"total"`
}

type DNSItem struct {
	Proto   string   `json:"proto"`
	Address string   `json:"address"`
	Domains []string `json:"domains"`
}

type Outbound struct {
	Name     string   `json:"name"`
	Protocol string   `json:"protocol"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`
	Password string   `json:"password"`
	Rules    []string `json:"rules"`
}

type Protocol struct {
	Type                    string `json:"type"`
	Port                    int    `json:"port"`
	Enable                  bool   `json:"enable"`
	Security                string `json:"security"`
	SNI                     string `json:"sni"`
	AllowInsecure           bool   `json:"allow_insecure"`
	Fingerprint             string `json:"fingerprint"`
	RealityServerAddr       string `json:"reality_server_addr"`
	RealityServerPort       int    `json:"reality_server_port"`
	RealityPrivateKey       string `json:"reality_private_key"`
	RealityPublicKey        string `json:"reality_public_key"`
	RealityShortID          string `json:"reality_short_id"`
	Transport               string `json:"transport"`
	Host                    string `json:"host"`
	Path                    string `json:"path"`
	ServiceName             string `json:"service_name"`
	Cipher                  string `json:"cipher"`
	ServerKey               string `json:"server_key"`
	Flow                    string `json:"flow"`
	HopPorts                string `json:"hop_ports"`
	HopInterval             int    `json:"hop_interval"`
	ObfsPassword            string `json:"obfs_password"`
	DisableSNI              bool   `json:"disable_sni"`
	ReduceRTT               bool   `json:"reduce_rtt"`
	UDPRelayMode            string `json:"udp_relay_mode"`
	CongestionController    string `json:"congestion_controller"`
	Multiplex               string `json:"multiplex"`
	PaddingScheme           string `json:"padding_scheme"`
	UpMbps                  int    `json:"up_mbps"`
	DownMbps                int    `json:"down_mbps"`
	Obfs                    string `json:"obfs"`
	ObfsHost                string `json:"obfs_host"`
	ObfsPath                string `json:"obfs_path"`
	XHTTPMode               string `json:"xhttp_mode"`
	XHTTPExtra              string `json:"xhttp_extra"`
	Encryption              string `json:"encryption"`
	EncryptionMode          string `json:"encryption_mode"`
	EncryptionRTT           string `json:"encryption_rtt"`
	EncryptionTicket        string `json:"encryption_ticket"`
	EncryptionServerPadding string `json:"encryption_server_padding"`
	EncryptionPrivateKey    string `json:"encryption_private_key"`
	EncryptionClientPadding string `json:"encryption_client_padding"`
	EncryptionPassword      string `json:"encryption_password"`
	CertMode                string `json:"cert_mode"`
	CertDNSProvider         string `json:"cert_dns_provider"`
	CertDNSEnv              string `json:"cert_dns_env"`
}

// ─── Node ─────────────────────────────────────────────────────────────────────

type NodeInfo struct {
	Id                     int
	Type                   string
	PushInterval           int
	PullInterval           int
	TrafficReportThreshold int
	Protocol               *Protocol
}

type NodeStatus struct {
	CPU    float64
	Mem    float64
	Disk   float64
	Uptime uint64
}

// ─── User ─────────────────────────────────────────────────────────────────────

type OnlineUser struct {
	UID int
	IP  string
}

type UserInfo struct {
	Id          int    `json:"id"`
	Uuid        string `json:"uuid"`
	SpeedLimit  int    `json:"speed_limit"`
	DeviceLimit int    `json:"device_limit"`
}

type UserListBody struct {
	Users []UserInfo `json:"users"`
}

type UserOnlineBody struct {
	Users []OnlineUser `json:"users"`
}

type AliveMap struct {
	Alive map[int]int `json:"alive"`
}

type UserTraffic struct {
	UID      int   `json:"uid"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}
