package watchguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Certificate struct {
	ID          string `json:"id"`
	Device      string `json:"device"`
	Name        string `json:"name"`
	Type        string `json:"cert_type"`
	Fingerprint string `json:"cert_fingerprint"`
	PEM         string `json:"pem"`
	Expiration  int64  `json:"expiration"`
	Inactive    bool   `json:"inactive"`
}

func (c *Client) Certificates(ctx context.Context, device string) ([]Certificate, error) {
	var raw json.RawMessage
	err := c.request(ctx, "GET", c.accountPath("/configuration/")+"/system/certificates?device="+url.QueryEscape(device), nil, &raw)
	if err != nil {
		return nil, err
	}
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, errors.New("empty certificate response")
	}
	if raw[0] == '[' {
		var a []Certificate
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, errors.New("invalid certificate list")
		}
		return a, nil
	}
	var envelope struct {
		Objects     []Certificate `json:"objects"`
		Data        []Certificate `json:"data"`
		ID          string        `json:"id"`
		Device      string        `json:"device"`
		Name        string        `json:"name"`
		Type        string        `json:"cert_type"`
		Fingerprint string        `json:"cert_fingerprint"`
		PEM         string        `json:"pem"`
		Expiration  int64         `json:"expiration"`
		Inactive    bool          `json:"inactive"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, errors.New("invalid certificate response")
	}
	if envelope.Objects != nil {
		return envelope.Objects, nil
	}
	if envelope.Data != nil {
		return envelope.Data, nil
	}
	if envelope.ID != "" {
		return []Certificate{{envelope.ID, envelope.Device, envelope.Name, envelope.Type, envelope.Fingerprint, envelope.PEM, envelope.Expiration, envelope.Inactive}}, nil
	}
	return nil, errors.New("unrecognized certificate list response")
}

func (c *Client) CreateCertificate(ctx context.Context, device, name, cert, key string) (Certificate, error) {
	var out Certificate
	err := c.request(ctx, "POST", c.accountPath("/configuration/")+"/system/certificates", map[string]string{"device": device, "name": name, "pem": cert, "pvt_key": key}, &out)
	if err != nil {
		return out, err
	}
	if out.ID == "" {
		return out, errors.New("create certificate response has no ID")
	}
	return out, nil
}

func (c *Client) InstallCertificate(ctx context.Context, device, id string) (Transaction, error) {
	var raw json.RawMessage
	err := c.request(ctx, "POST", c.accountPath("/commands/")+"/system/certificates/install", map[string]any{"certificate_id": id, "devices": []string{device}}, &raw)
	if err != nil {
		return Transaction{}, err
	}
	out, err := decodeTransaction(raw)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid install response: %w", err)
	}
	if failedTransactionStatus(out.Status) {
		return out, fmt.Errorf("install command %s ended with status %s", out.ID, out.Status)
	}
	return out, nil
}

type Transaction struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Device json.RawMessage `json:"device,omitempty"`
	Type   string          `json:"type,omitempty"`
}

func decodeTransaction(raw json.RawMessage) (Transaction, error) {
	var one Transaction
	if err := json.Unmarshal(raw, &one); err == nil && one.ID != "" {
		return one, nil
	}
	var list []Transaction
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 && list[0].ID != "" {
		return list[0], nil
	}
	return Transaction{}, errors.New("response has no transaction ID")
}

func failedTransactionStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "failure", "canceled", "cancelled", "timed_out", "timeout", "error":
		return true
	default:
		return false
	}
}

func completeTransactionStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "complete", "completed", "confirmed", "success", "succeeded":
		return true
	default:
		return false
	}
}

func (c *Client) WaitTransaction(ctx context.Context, id string, pollInterval time.Duration) (Transaction, error) {
	if id == "" {
		return Transaction{}, errors.New("transaction ID is required")
	}
	if pollInterval <= 0 {
		pollInterval = 3 * time.Second
	}
	for {
		tx, err := c.Transaction(ctx, id)
		if err != nil {
			return Transaction{}, err
		}
		if failedTransactionStatus(tx.Status) {
			return tx, fmt.Errorf("transaction %s ended with status %s", tx.ID, tx.Status)
		}
		if completeTransactionStatus(tx.Status) {
			return tx, nil
		}
		select {
		case <-ctx.Done():
			return tx, fmt.Errorf("transaction %s did not complete: %w", id, ctx.Err())
		case <-time.After(pollInterval):
		}
	}
}

func deploymentDevice(id string) any {
	if strings.HasPrefix(id, "FB-") {
		if n, err := strconv.ParseInt(strings.TrimPrefix(id, "FB-"), 10, 64); err == nil {
			return n
		}
	}
	return id
}

func (c *Client) DeployConfiguration(ctx context.Context, device string) (Transaction, error) {
	var raw json.RawMessage
	err := c.request(ctx, "POST", c.accountPath("/commands/")+"/deployments", map[string]any{"start_date": "now", "description": "wgcert certificate deployment", "devices": []any{deploymentDevice(device)}, "full_config": true, "staged": false}, &raw)
	if err != nil {
		return Transaction{}, err
	}
	out, err := decodeTransaction(raw)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid deployment response: %w", err)
	}
	if failedTransactionStatus(out.Status) {
		return out, fmt.Errorf("deployment %s ended with status %s", out.ID, out.Status)
	}
	return out, nil
}

func (c *Client) Transaction(ctx context.Context, id string) (Transaction, error) {
	var t Transaction
	err := c.request(ctx, "GET", c.accountPath("/commands/")+"/transactions/"+url.PathEscape(id), nil, &t)
	if err != nil {
		return t, err
	}
	if t.ID == "" {
		return t, fmt.Errorf("transaction %s was not returned", id)
	}
	return t, nil
}
