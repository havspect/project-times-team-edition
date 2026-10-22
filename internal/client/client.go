// Package client is the typed HTTP client the CLI and MCP server share.
package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const apiPrefix = "/api/v1"

// Config says which server to talk to and, when set, which token to send.
type Config struct {
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

// Client talks to one Project Times server. Login keeps its session cookie in the
// jar; a Config.Token is sent as a bearer header on every request.
type Client struct {
	cfg  Config
	http *http.Client
}

// New returns a client for cfg with a cookie jar, so Login can keep its session.
func New(cfg Config) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{cfg: cfg, http: &http.Client{Jar: jar}}
}

// Me is the authenticated user.
type Me struct {
	ID      uint   `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
}

// Token is a personal API token; Token holds the raw value only in the reply to
// CreateToken.
type Token struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Token      string     `json:"token,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// Error is a non-2xx reply from the server.
type Error struct {
	StatusCode int
	Detail     string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return http.StatusText(e.StatusCode)
	}
	return http.StatusText(e.StatusCode) + ": " + e.Detail
}

// Login authenticates with a password and keeps the session cookie for later calls.
func (c *Client) Login(email, password string) (*Me, error) {
	var me Me
	body := map[string]string{"email": email, "password": password}
	if err := c.do(http.MethodPost, "/auth/login", body, &me); err != nil {
		return nil, err
	}
	return &me, nil
}

// Me returns the user the credentials belong to.
func (c *Client) Me() (*Me, error) {
	var me Me
	if err := c.do(http.MethodGet, "/me", nil, &me); err != nil {
		return nil, err
	}
	return &me, nil
}

// CreateToken mints a personal API token; the raw value is in the reply only.
func (c *Client) CreateToken(name string) (*Token, error) {
	var t Token
	if err := c.do(http.MethodPost, "/me/tokens", map[string]string{"name": name}, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (c *Client) do(method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, strings.TrimSuffix(c.cfg.URL, "/")+apiPrefix+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	if method != http.MethodGet {
		// Cookie-authenticated mutations need a same-origin Origin; harmless on bearer requests.
		req.Header.Set("Origin", c.origin())
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return serverError(resp)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// origin is the scheme://host the server compares its Origin header against.
func (c *Client) origin() string {
	u, err := url.Parse(c.cfg.URL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func serverError(resp *http.Response) error {
	var body struct {
		Detail string `json:"detail"`
	}
	// A non-JSON error body still has a status, which is the useful part.
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return &Error{StatusCode: resp.StatusCode, Detail: body.Detail}
}
