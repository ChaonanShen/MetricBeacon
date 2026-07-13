package grafana

// Write is a reserved replacement point for future approval-gated Grafana writes.
type Write interface{ PrepareWrite() error }
