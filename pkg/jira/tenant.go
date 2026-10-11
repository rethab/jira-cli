package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// TenantInfo holds tenant info of a Jira Cloud site.
type TenantInfo struct {
	CloudID string `json:"cloudId"`
}

// FetchCloudID looks up the cloud ID of a Jira Cloud site. Scoped API tokens are only
// accepted by the Atlassian API gateway, which addresses sites by cloud ID.
func FetchCloudID(client *http.Client, site string) (string, error) {
	endpoint := strings.TrimRight(site, "/") + "/_edge/tenant_info"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("requesting %s: %w", endpoint, err)
	}

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting %s: %w", endpoint, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("requesting %s: unexpected status %s", endpoint, res.Status)
	}

	var info TenantInfo
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return "", fmt.Errorf("decoding response from %s: %w", endpoint, err)
	}
	if info.CloudID == "" {
		return "", fmt.Errorf("response from %s contains no cloud ID", endpoint)
	}

	return info.CloudID, nil
}
