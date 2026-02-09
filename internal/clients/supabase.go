package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
)

type SupabaseClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewSupabaseClient(projectID, apiKey string) *SupabaseClient {
	return &SupabaseClient{
		BaseURL:    fmt.Sprintf("https://%s.supabase.co/rest/v1", projectID),
		APIKey:     apiKey,
		HTTPClient: xray.Client(&http.Client{Timeout: 10 * time.Second}),
	}
}

type OAuthToken struct {
	AccessToken string `json:"access_token"`
	Email       string `json:"email"`
}

func (c *SupabaseClient) GetAccessToken(ctx context.Context, email string) (string, error) {
	url := fmt.Sprintf("%s/oauth_tokens?email=eq.%s&select=access_token", c.BaseURL, email)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("apikey", c.APIKey)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Supabase API error: %s", resp.Status)
	}

	var tokens []OAuthToken
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return "", err
	}

	if len(tokens) == 0 {
		return "", fmt.Errorf("no token found for email: %s", email)
	}

	return tokens[0].AccessToken, nil
}
