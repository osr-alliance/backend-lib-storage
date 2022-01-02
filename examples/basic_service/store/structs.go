package store

import "time"

type Leads struct {
	LeadID    int32     `json:"lead_id"`
	UserID    int32     `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}
