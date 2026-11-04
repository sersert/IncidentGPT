package main

import "time"

type GraphNode struct {
	ID        string    `json:"id"`
	ClusterID string    `json:"cluster_id"`
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	LastSeen  time.Time `json:"last_seen"`
	Status    string    `json:"status"`
}
type GraphEdge struct {
	ID           string    `json:"id"`
	From         string    `json:"from"`
	To           string    `json:"to"`
	Kind         string    `json:"kind"`
	Source       string    `json:"source"`
	Observed     bool      `json:"observed"`
	EvidenceRef  string    `json:"evidence_ref"`
	LastSeen     time.Time `json:"last_seen"`
	StaleSeconds int       `json:"stale_seconds"`
}
type GraphDocument struct {
	Version   int         `json:"schema_version"`
	ClusterID string      `json:"cluster_id"`
	Nodes     []GraphNode `json:"nodes"`
	Edges     []GraphEdge `json:"edges"`
}
