package watchguard

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type Device struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Model        string `json:"short_model_name"`
	Version      string `json:"version"`
	CloudManaged string `json:"cloud_managed"`
	State        string `json:"state"`
}

func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	return c.DevicesWithHierarchy(ctx, false)
}

func (c *Client) DevicesWithHierarchy(ctx context.Context, includeHierarchy bool) ([]Device, error) {
	var r struct {
		Data []Device `json:"data"`
	}
	p := c.accountPath("/info/") + "/devices"
	q := url.Values{}
	if includeHierarchy {
		// WatchGuard does not allow include_hierarchy together with device_type.
		// For Service Provider accounts this includes Fireboxes from Subscriber accounts.
		q.Set("include_hierarchy", "true")
	} else {
		q.Set("device_type", "FB")
	}
	p += "?" + q.Encode()
	err := c.request(ctx, "GET", p, nil, &r)
	return r.Data, err
}

func (c *Client) Device(ctx context.Context, id string) (Device, error) {
	var r struct {
		Data []Device `json:"data"`
	}
	p := c.accountPath("/info/") + "/devices?device=" + url.QueryEscape(id)
	err := c.request(ctx, "GET", p, nil, &r)
	if err != nil {
		return Device{}, err
	}
	if len(r.Data) != 1 {
		return Device{}, fmt.Errorf("device %s not found", id)
	}
	if !strings.EqualFold(r.Data[0].CloudManaged, "yes") {
		return Device{}, fmt.Errorf("device %s is locally managed; certificate API requires a cloud-managed Firebox", id)
	}
	return r.Data[0], nil
}
