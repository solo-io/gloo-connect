package api

type AgentConnect struct {
	c *Client
}

func NewAgentConnect(c *Client) *AgentConnect {
	return &AgentConnect{c: c}
}

type RootCA struct {
	ID       string
	RootCert string
	Active   bool
}

type RootsInfo struct {
	ActiveRootID string
	Roots        []RootCA
}
type ProxyConfig struct {
	BindAddress         string     `json:"bind_address"`
	BindPort            uint       `json:"bind_port"`
	LocalServiceAddress string     `json:"local_service_address"`
	Upstreams           []Upstream `json:"upstreams"`
}

type Upstream struct {
	DestinationName string `json:"destination_name"`
	DestinationType string `json:"destination_type"`
	LocalBindPort   string `json:"local_bind_port"`
}

type ProxyInfo struct {
	ProxyServiceID    string
	TargetServiceID   string
	TargetServiceName string
	ContentHash       string
	ExecMode          string
	Command           []string
	Config            ProxyConfig
}

func (c *AgentConnect) doGet(q *QueryOptions, url string, out interface{}) (*QueryMeta, error) {
	r := c.c.newRequest("GET", url)
	r.setQueryOptions(q)
	rtt, resp, err := requireOK(c.c.doRequest(r))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	qm := &QueryMeta{}
	parseQueryMeta(resp, qm)
	qm.RequestTime = rtt

	if err := decodeBody(resp, out); err != nil {
		return nil, err
	}
	return qm, nil

}

type LeafCertInfo struct {
	SerialNumber  string
	CertPEM       string
	PrivateKeyPEM string
	Service       string
	ServiceURI    string
	ValidAfter    string
	ValidBefore   string
}

func (c *AgentConnect) RootCerts(q *QueryOptions) (*RootsInfo, *QueryMeta, error) {
	var out RootsInfo
	qm, err := c.doGet(q, "/v1/agent/connect/ca/roots", &out)
	if err != nil {
		return nil, nil, err
	}
	return &out, qm, nil
}

func (c *AgentConnect) LeafCert(svcname string, q *QueryOptions) (*LeafCertInfo, *QueryMeta, error) {
	var out LeafCertInfo
	qm, err := c.doGet(q, "/v1/agent/connect/ca/leaf/"+svcname, &out)
	if err != nil {
		return nil, nil, err
	}
	return &out, qm, nil
}

func (c *AgentConnect) ProxyConfig(proxyid string, q *QueryOptions) (*ProxyInfo, *QueryMeta, error) {
	var out ProxyInfo
	qm, err := c.doGet(q, "/agent/connect/proxy/"+proxyid, &out)
	if err != nil {
		return nil, nil, err
	}
	return &out, qm, nil
}
