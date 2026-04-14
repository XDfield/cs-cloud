package model

type Event struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	MessageID      string `json:"msg_id,omitempty"`
	Backend        string `json:"backend,omitempty"`
	Data           any    `json:"data"`
}
