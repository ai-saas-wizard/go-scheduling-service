package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
)

type SearchClient struct {
	SearchLambdaURL string
	HTTPClient      *http.Client
}

func NewSearchClient(url string) *SearchClient {
	return &SearchClient{
		SearchLambdaURL: url,
		HTTPClient:      xray.Client(&http.Client{Timeout: 15 * time.Second}),
	}
}

type SearchResponse struct {
	Count   int            `json:"count"`
	Results []SearchResult `json:"results"`
}

type SearchResult struct {
	PropertyID string                 `json:"property_id"`
	Metadata   map[string]interface{} `json:"metadata"`
}

func (c *SearchClient) FindPropertyID(ctx context.Context, query string) (string, error) {
	body := map[string]string{
		"Query":             query,
		"ExtractedProperty": query,
	}
	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.SearchLambdaURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search service error: %s", resp.Status)
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Results) == 0 {
		return "", fmt.Errorf("no property found for query: %s", query)
	}

	firstResult := result.Results[0]
	meta := firstResult.Metadata

	if val, ok := meta["PropertyId"]; ok {
		return fmt.Sprintf("%v", val), nil
	}
	if val, ok := meta["property_id"]; ok {
		return fmt.Sprintf("%v", val), nil
	}

	if firstResult.PropertyID != "" {
		return firstResult.PropertyID, nil
	}

	if val, ok := meta["Id"]; ok {
		return fmt.Sprintf("%v", val), nil
	}
	if val, ok := meta["id"]; ok {
		return fmt.Sprintf("%v", val), nil
	}

	return "", fmt.Errorf("property ID missing in search result")
}
