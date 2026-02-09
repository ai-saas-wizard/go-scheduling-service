package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/vishnuanilkumar/go-scheduling-service/internal/models"
)

type AppFolioClient struct {
	BaseURL     string
	AuthHeader  string
	DeveloperID string
	HTTPClient  *http.Client
}

func NewAppFolioClient(authHeader, developerID string) *AppFolioClient {
	return &AppFolioClient{
		BaseURL:     "https://api.appfolio.com",
		AuthHeader:  authHeader,
		DeveloperID: developerID,
		HTTPClient:  xray.Client(&http.Client{Timeout: 10 * time.Second}),
	}
}

func (c *AppFolioClient) GetProperty(ctx context.Context, propertyID string) (*models.AppFolioProperty, error) {
	url := fmt.Sprintf("%s/api/v0/properties?filters[Id]=%s", c.BaseURL, propertyID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AppFolio API error (Property): %s", resp.Status)
	}

	var result models.AppFolioPropertyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("property not found: %s", propertyID)
	}

	return &result.Data[0], nil
}

func (c *AppFolioClient) GetPropertyGroups(ctx context.Context, ids []string) ([]models.AppFolioGroup, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	idsStr := strings.Join(ids, ",")
	url := fmt.Sprintf("%s/api/v0/property_groups?filters[Id]=%s", c.BaseURL, idsStr)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AppFolio API error (Groups): %s", resp.Status)
	}

	var result models.AppFolioGroupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func (c *AppFolioClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", c.AuthHeader)
	req.Header.Set("X-AppFolio-Developer-ID", c.DeveloperID)
}
