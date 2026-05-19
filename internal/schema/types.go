package schema

// Status represents the lifecycle stage of any network entity.
type Status string

const (
	StatusConcept   Status = "concept"
	StatusPlanned   Status = "planned"
	StatusConfirmed Status = "confirmed"
	StatusActive    Status = "active"
)

// Device is a network node (router, switch, firewall, etc.).
type Device struct {
	Name   string `yaml:"name"`
	Role   string `yaml:"role"`
	Vendor string `yaml:"vendor"`
	Model  string `yaml:"model"`
	Site   string `yaml:"site"`
	Status Status `yaml:"status"`
	// Count is set by the HLD adapter for concept-mode role-group nodes.
	// A count > 1 means this node represents N physical devices of the same role.
	Count int `yaml:"count,omitempty"`
}

// Site is a physical or logical location that groups devices.
type Site struct {
	Name   string `yaml:"name"`
	Type   string `yaml:"type"`
	Region string `yaml:"region"`
	Status Status `yaml:"status"`
}

// Region is a geographic grouping of sites.
type Region struct {
	Name   string `yaml:"name"`
	Status Status `yaml:"status"`
}

// PhysicalLink is a cabled connection between two device interfaces.
type PhysicalLink struct {
	AEnd       string `yaml:"a_end"`
	ZEnd       string `yaml:"z_end"`
	AInterface string `yaml:"a_interface,omitempty"`
	ZInterface string `yaml:"z_interface,omitempty"`
	Kind       string `yaml:"kind"`
	Role       string `yaml:"role"`
	Speed      string `yaml:"speed,omitempty"`
	Source     string `yaml:"source,omitempty"`
	Status     Status `yaml:"status"`
	LagID      string `yaml:"lag_id,omitempty"`
}

// LogicalLink is a protocol-level connection derived from device configuration.
type LogicalLink struct {
	AEnd     string `yaml:"a_end"`
	ZEnd     string `yaml:"z_end"`
	Kind     string `yaml:"kind"`
	Role     string `yaml:"role"`
	Protocol string `yaml:"protocol,omitempty"`
	Source   string `yaml:"source,omitempty"`
	Status   Status `yaml:"status"`
}
