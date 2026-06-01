package panel

import (
	"context"
	"encoding/json"
	"fmt"
)

type certPinResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    bool   `json:"data"`
}

// ReportCertPin uploads leaf cert SHA256 to v2board after generate/renew.
func (c *Client) ReportCertPin(ctx context.Context, pin string) error {
	const path = "/api/v2/server/cert-pin"
	r, err := c.client.R().
		SetContext(ctx).
		SetBody(map[string]string{
			"pinned_peer_cert_sha256": pin,
		}).
		ForceContentType("application/json").
		Post(path)
	if err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("received nil response")
	}
	if r.StatusCode() >= 400 {
		return fmt.Errorf("report cert pin http %d: %s", r.StatusCode(), r.String())
	}
	var resp certPinResponse
	if err := json.Unmarshal(r.Body(), &resp); err != nil {
		return fmt.Errorf("decode cert pin response: %w", err)
	}
	if resp.Status != "success" {
		if resp.Message != "" {
			return fmt.Errorf("report cert pin failed: %s", resp.Message)
		}
		return fmt.Errorf("report cert pin failed")
	}
	return nil
}
