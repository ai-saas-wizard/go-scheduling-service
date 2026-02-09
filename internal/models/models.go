package models

import (
	"encoding/json"
	"time"
)

// Request is the input event for the Lambda
type Request struct {
	Query string `json:"Query"`
	Phone string `json:"Phone,omitempty"`
}

// Response is the output of the Lambda
type Response struct {
	Success      bool         `json:"success"`
	Property     PropertyInfo `json:"property"`
	Agent        AgentInfo    `json:"agent"`
	Availability Availability `json:"availability"`
	Message      string       `json:"message"`
	FormattedMsg string       `json:"formattedMessage"`
}

type PropertyInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address,omitempty"`
	City    string `json:"city,omitempty"`
	State   string `json:"state,omitempty"`
}

type AgentInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Zone      string `json:"zone,omitempty"`
	ZoneGroup string `json:"zoneGroup,omitempty"`
}

type Availability struct {
	TotalSlotsAvailable int        `json:"totalSlotsAvailable"`
	DaysChecked         int        `json:"daysChecked"`
	Slots               []TimeSlot `json:"slots"`
}

type TimeSlot struct {
	Date  string    `json:"date"`  // "Friday, December 6, 2025"
	Time  string    `json:"time"`  // "9:00 AM"
	Start time.Time `json:"start"` // ISO string
	End   time.Time `json:"end"`   // ISO string
}

// --- AppFolio Models ---

type AppFolioPropertyResponse struct {
	Data []AppFolioProperty `json:"data"`
}

type AppFolioProperty struct {
	ID               string   `json:"Id"`
	Name             string   `json:"Name"`
	Address1         string   `json:"Address1"`
	City             string   `json:"City"`
	State            string   `json:"State"`
	PropertyGroupIds []string `json:"PropertyGroupIds"`
}

type AppFolioGroupResponse struct {
	Data []AppFolioGroup `json:"data"`
}

type AppFolioGroup struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

// --- Google Calendar Models ---

type FreeBusyRequest struct {
	TimeMin  string            `json:"timeMin"`
	TimeMax  string            `json:"timeMax"`
	TimeZone string            `json:"timeZone"`
	Items    []FreeBusyReqItem `json:"items"`
}

type FreeBusyReqItem struct {
	ID string `json:"id"`
}

type FreeBusyResponse struct {
	Calendars map[string]FreeBusyCalendar `json:"calendars"`
}

type FreeBusyCalendar struct {
	Busy   []TimeRange `json:"busy"`
	Errors []Error     `json:"errors,omitempty"`
}

type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Error struct {
	Domain string `json:"domain"`
	Reason string `json:"reason"`
}

// --- VAPI Webhook Models ---

// VAPIWebhookPayload represents the incoming VAPI webhook request
type VAPIWebhookPayload struct {
	Message VAPIMessage `json:"message"`
}

type VAPIMessage struct {
	Type      string         `json:"type"`
	ToolCalls []VAPIToolCall `json:"toolCalls"`
	Artifact  VAPIArtifact   `json:"artifact"`
}

type VAPIToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function VAPIToolFunction `json:"function"`
}

type VAPIToolFunction struct {
	Name      string           `json:"name"`
	Arguments VAPIFunctionArgs `json:"arguments"`
}

type VAPIFunctionArgs struct {
	Query             string `json:"Query"`
	Phone             string `json:"Phone"`
	ExtractedProperty string `json:"ExtractedProperty,omitempty"`
}

type VAPIArtifact struct {
	Messages []VAPIArtifactMessage `json:"messages"`
}

type VAPIArtifactMessage struct {
	Role      string          `json:"role"`
	Name      string          `json:"name,omitempty"`
	RawResult json.RawMessage `json:"result,omitempty"`
}

// ParseResult attempts to parse the result as a VAPIToolCallResult.
// Returns nil if the result is a string or cannot be parsed.
func (m *VAPIArtifactMessage) ParseResult() *VAPIToolCallResult {
	if len(m.RawResult) == 0 {
		return nil
	}
	// Skip if the result is a JSON string (starts with '"')
	if m.RawResult[0] == '"' {
		return nil
	}
	var result VAPIToolCallResult
	if err := json.Unmarshal(m.RawResult, &result); err != nil {
		return nil
	}
	return &result
}

type VAPIToolCallResult struct {
	Count   int                  `json:"count"`
	Results []VAPIPropertyResult `json:"results"`
}

type VAPIPropertyResult struct {
	ID         string               `json:"id"`
	PropertyID string               `json:"property_id"`
	Content    string               `json:"content"`
	Metadata   VAPIPropertyMetadata `json:"metadata"`
}

type VAPIPropertyMetadata struct {
	Address1   string `json:"Address1"`
	Address2   string `json:"Address2,omitempty"`
	City       string `json:"City"`
	State      string `json:"State"`
	PropertyId string `json:"PropertyId"` // Parent property ID for AppFolio
	UnitId     string `json:"UnitId"`
}
