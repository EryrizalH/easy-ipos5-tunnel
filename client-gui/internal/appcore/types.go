package appcore

type StatusSnapshot struct {
	ServiceName          string `json:"serviceName"`
	ServiceState         string `json:"serviceState"`
	ConnectionState      string `json:"connectionState"`
	ServerHost           string `json:"serverHost"`
	ServerControlPort    string `json:"serverControlPort"`
	ServerPublicIP       string `json:"serverPublicIP"`
	DashboardReachable   bool   `json:"dashboardReachable"`
	ControlPortReachable bool   `json:"controlPortReachable"`
	PostgresStatus       string `json:"postgresStatus"`
	PostgresConnectMs    string `json:"postgresConnectMs"`
	PostgresQueryMs      string `json:"postgresQueryMs"`
	PostgresTxMs         string `json:"postgresTxMs"`
	PostgresLastChecked  string `json:"postgresLastChecked"`
	PostgresLastError    string `json:"postgresLastError"`
	LastCheckedAt        string `json:"lastCheckedAt"`
	LastError            string `json:"lastError"`
}

type ActionResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}
