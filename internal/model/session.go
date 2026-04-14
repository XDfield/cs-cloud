package model

type Session struct {
	ID        string `json:"id"`
	Backend   string `json:"backend"`
	Driver    string `json:"driver"`
	State     string `json:"state"`
	SessionID string `json:"session_id,omitempty"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}
