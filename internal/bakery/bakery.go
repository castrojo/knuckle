package bakery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/castrojo/knuckle/internal/model"
)

const (
	// DefaultCatalogURL is the Flatcar Bakery sysext catalog endpoint
	DefaultCatalogURL = "https://www.flatcar.org/api/sysext-catalog.json"
	defaultTimeout    = 30 * time.Second
)

// Client is the interface for fetching the sysext catalog
type Client interface {
	FetchCatalog(ctx context.Context) ([]model.SysextEntry, error)
}

// HTTPClient fetches the catalog from the Flatcar Bakery API
type HTTPClient struct {
	CatalogURL string
	HTTP       *http.Client
}

// NewHTTPClient creates a new bakery HTTP client
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		CatalogURL: DefaultCatalogURL,
		HTTP: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// NewHTTPClientWithURL creates a client pointing at a custom catalog URL (for testing)
func NewHTTPClientWithURL(url string) *HTTPClient {
	return &HTTPClient{
		CatalogURL: url,
		HTTP: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// catalogEntry matches the expected JSON schema from the Flatcar Bakery API
type catalogEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	URL         string `json:"url"`
}

func (c *HTTPClient) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.CatalogURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "knuckle/1.0")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching catalog: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog returned status %d", resp.StatusCode)
	}

	// Limit response body to 5MB to prevent OOM from malicious/broken responses.
	const maxResponseSize = 5 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var entries []catalogEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing catalog JSON: %w", err)
	}

	sysexts := make([]model.SysextEntry, 0, len(entries))
	for _, e := range entries {
		sysexts = append(sysexts, model.SysextEntry{
			Name:        e.Name,
			Description: e.Description,
			Version:     e.Version,
			URL:         e.URL,
			Selected:    false,
		})
	}

	return sysexts, nil
}

// MockClient is a test double that returns preconfigured results
type MockClient struct {
	Entries []model.SysextEntry
	Err     error
}

func (m *MockClient) FetchCatalog(ctx context.Context) ([]model.SysextEntry, error) {
	return m.Entries, m.Err
}
