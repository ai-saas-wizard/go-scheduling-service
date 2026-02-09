package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/vishnuanilkumar/go-scheduling-service/internal/models"
)

type CalendarClient struct {
	HTTPClient *http.Client
}

func NewCalendarClient() *CalendarClient {
	return &CalendarClient{
		HTTPClient: xray.Client(&http.Client{Timeout: 15 * time.Second}),
	}
}

func (c *CalendarClient) GetBusySlots(ctx context.Context, accessToken, email string, timeMin, timeMax time.Time) ([]models.TimeRange, error) {
	url := "https://www.googleapis.com/calendar/v3/freeBusy"

	reqBody := models.FreeBusyRequest{
		TimeMin:  timeMin.Format(time.RFC3339),
		TimeMax:  timeMax.Format(time.RFC3339),
		TimeZone: "America/Los_Angeles",
		Items:    []models.FreeBusyReqItem{{ID: email}},
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google Calendar API error: %s", resp.Status)
	}

	var result models.FreeBusyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	calendar, ok := result.Calendars[email]
	if !ok {
		return nil, fmt.Errorf("calendar not found in response for %s", email)
	}

	if len(calendar.Errors) > 0 {
		return nil, fmt.Errorf("calendar error: %s", calendar.Errors[0].Reason)
	}

	return calendar.Busy, nil
}
