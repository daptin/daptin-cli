package src

type DaptinHostEndpoint struct {
	Name     string
	Endpoint string
	Token    string
}

type DaptinCliConfig struct {
	CurrentContextName string
	Context            DaptinHostEndpoint `json:"-"`
	Hosts              []DaptinHostEndpoint
}
