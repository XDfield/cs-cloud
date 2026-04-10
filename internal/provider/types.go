package provider

type Credentials struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	State        string `json:"state,omitempty"`
	MachineID    string `json:"machine_id"`
	BaseURL      string `json:"base_url"`
	ExpiryDate   int64  `json:"expiry_date"`
	UpdatedAt    string `json:"updated_at"`
	ExpiredAt    string `json:"expired_at,omitempty"`
}
