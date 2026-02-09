package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/vishnuanilkumar/go-scheduling-service/internal/ratelimit"
)

type OpenAIClient struct {
	APIKey     string
	HTTPClient *http.Client
}

func NewOpenAIClient(apiKey string) *OpenAIClient {
	return &OpenAIClient{
		APIKey:     apiKey,
		HTTPClient: xray.Client(&http.Client{Timeout: 30 * time.Second}),
	}
}

// AddressCandidate represents a property address option
type AddressCandidate struct {
	Index      int
	Address1   string
	PropertyId string
}

// MatchAddressToQuery uses OpenAI to find the best matching address for a query
func (c *OpenAIClient) MatchAddressToQuery(ctx context.Context, query string, candidates []AddressCandidate) (string, error) {
	if len(candidates) == 0 {
		return "", fmt.Errorf("no address candidates provided")
	}

	// Rate limit check
	if err := ratelimit.WaitForOpenAI(ctx); err != nil {
		return "", err
	}

	// Build the prompt
	addressList := ""
	for i, cand := range candidates {
		addressList += fmt.Sprintf("%d. %s\n", i, cand.Address1)
	}

	prompt := fmt.Sprintf(`Given the user's spoken query about a property address, find the best matching address from the list.

User Query: "%s"

Available Addresses:
%sReturn ONLY the index number (0, 1, 2, etc.) of the best matching address. If no address matches at all, return -1.

Important: The query may contain spoken numbers (like "eight twenty eight" for "828") or slight variations. Match based on the most likely intended address.`, query, addressList)

	// OpenAI API request
	reqBody := map[string]interface{}{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  10,
		"temperature": 0,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API error: %s", resp.Status)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	// Parse the index from response
	content := result.Choices[0].Message.Content
	var matchedIndex int
	if _, err := fmt.Sscanf(content, "%d", &matchedIndex); err != nil {
		return "", fmt.Errorf("failed to parse OpenAI response: %s", content)
	}

	if matchedIndex < 0 || matchedIndex >= len(candidates) {
		return "", fmt.Errorf("no matching address found")
	}

	return candidates[matchedIndex].PropertyId, nil
}
