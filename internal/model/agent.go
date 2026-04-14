package model

type AgentInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Driver      string `json:"driver"`
	Available   bool   `json:"available"`
	AuthRequired bool  `json:"auth_required,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
}
