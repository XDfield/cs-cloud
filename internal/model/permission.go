package model

type Permission struct {
	ID      string             `json:"id"`
	CallID  string             `json:"call_id"`
	Title   string             `json:"title"`
	Kind    string             `json:"kind"`
	Options []PermissionOption `json:"options"`
}

type PermissionOption struct {
	OptionID string `json:"option_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}
