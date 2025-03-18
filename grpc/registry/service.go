package registry

type Service struct {
	Name     string            `json:"name,omitempty"`
	Version  string            `json:"version,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	// Endpoints are endpoint addresses of the service instance.
	// schema:
	//   http://127.0.0.1:8000?isSecure=false
	//   grpc://127.0.0.1:9000?isSecure=false
	Endpoints []*Endpoint `json:"endpoints,omitempty"`
	Nodes     []*Node     `json:"nodes,omitempty"`
}

type Node struct {
	Id       string            `json:"id,omitempty"`
	Address  string            `json:"address,omitempty"`
	Port     int               `json:"port,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Endpoint struct {
	Name     string            `json:"name,omitempty"`
	Request  *Value            `json:"request,omitempty"`
	Response *Value            `json:"response,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Value struct {
	Name   string   `json:"name,omitempty"`
	Type   string   `json:"type,omitempty"`
	Values []*Value `json:"values,omitempty"`
}
