package watchguard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/eliazv/watchguard-acme-deploy/internal/config"
)

const prefix = "/rest/firebox/management/v1"

type Client struct {
	cfg    config.Config
	HTTP   *http.Client
	mu     sync.Mutex
	token  string
	expiry time.Time
}

func New(c config.Config) *Client {
	return &Client{cfg: c, HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) accountPath(part string) string {
	return prefix + part + url.PathEscape(c.cfg.AccountID)
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Until(c.expiry) > time.Minute {
		return c.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "scope": {"api-access"}}
	authURL := strings.TrimRight(c.cfg.AuthURL, "/")
	if !strings.HasSuffix(authURL, "/oauth/token") {
		authURL += "/oauth/token"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("invalid authentication URL")
	}
	req.SetBasicAuth(c.cfg.AccessID, c.cfg.AccessPassword)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", errors.New("authentication request failed")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("authentication rejected (HTTP %d)", res.StatusCode)
	}
	var v struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&v); err != nil || v.AccessToken == "" {
		return "", errors.New("invalid authentication response")
	}
	if v.ExpiresIn <= 0 {
		v.ExpiresIn = 3600
	}
	c.token = v.AccessToken
	c.expiry = time.Now().Add(time.Duration(v.ExpiresIn) * time.Second)
	return c.token, nil
}
func (c *Client) invalidate() { c.mu.Lock(); c.token = ""; c.mu.Unlock() }
func (c *Client) request(ctx context.Context, method, path string, in, out any) error {
	var body []byte
	var err error
	if in != nil {
		body, err = json.Marshal(in)
		if err != nil {
			return err
		}
	}
	attempts := 1
	if method == http.MethodGet {
		attempts = 3
	}
	for n := 0; n < attempts; n++ {
		token, err := c.accessToken(ctx)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.cfg.APIURL, "/")+path, bytes.NewReader(body))
		if err != nil {
			return errors.New("invalid API URL")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("WatchGuard-API-Key", c.cfg.APIKey)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		res, err := c.HTTP.Do(req)
		if err != nil {
			return fmt.Errorf("%s request failed: %w", method, err)
		}
		if res.StatusCode == http.StatusUnauthorized && method == http.MethodGet && n+1 < attempts {
			res.Body.Close()
			c.invalidate()
			continue
		}
		if (res.StatusCode == 429 || res.StatusCode >= 500) && method == http.MethodGet && n+1 < attempts {
			res.Body.Close()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(n+1) * 250 * time.Millisecond):
			}
			continue
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			res.Body.Close()
			return fmt.Errorf("WatchGuard API %s failed (HTTP %d)", method, res.StatusCode)
		}
		if out == nil {
			res.Body.Close()
			return nil
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
		res.Body.Close()
		if err != nil {
			return errors.New("could not read API response")
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return errors.New("empty API response")
		}
		if err := json.Unmarshal(data, out); err != nil {
			return errors.New("invalid API JSON response")
		}
		return nil
	}
	return errors.New("API request retries exhausted")
}
