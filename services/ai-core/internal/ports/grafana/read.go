package grafana

// Read is a reserved replacement point for future Grafana read integration.
type Read interface{ ReadDashboard() error }
