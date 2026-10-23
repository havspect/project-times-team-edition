package client

import "net/http"

// User is a user as the admin screens see it.
type User struct {
	ID                 uint   `json:"id"`
	Email              string `json:"email"`
	Name               string `json:"name"`
	Timezone           string `json:"timezone"`
	IsAdmin            bool   `json:"is_admin"`
	Active             bool   `json:"active"`
	MustChangePassword bool   `json:"must_change_password"`
}

// ListUsers returns every user; admin only.
func (c *Client) ListUsers() ([]User, error) {
	var out []User
	if err := c.do(http.MethodGet, "/users", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
