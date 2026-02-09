package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/vishnuanilkumar/go-scheduling-service/internal/clients"
	"github.com/vishnuanilkumar/go-scheduling-service/internal/logging"
	"github.com/vishnuanilkumar/go-scheduling-service/internal/logic"
	"github.com/vishnuanilkumar/go-scheduling-service/internal/models"
)

// GenericEvent handles multiple event formats (API Gateway, Function URL, direct invoke)
type GenericEvent struct {
	// Direct invocation fields
	Query string `json:"Query"`
	Phone string `json:"Phone"`

	// API Gateway / Function URL fields
	Body            string `json:"body"`
	IsBase64Encoded bool   `json:"isBase64Encoded"`
}

// LambdaResponse wraps the output for API Gateway compatibility
type LambdaResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

func init() {
	logging.Init()
	xray.Configure(xray.Config{
		LogLevel: "warn",
	})
}

func HandleRequest(ctx context.Context, event json.RawMessage) (LambdaResponse, error) {
	start := time.Now()

	// Extract Lambda request ID
	requestID := "unknown"
	if lc, ok := lambdacontext.FromContext(ctx); ok && lc != nil {
		requestID = lc.AwsRequestID
	}
	ctx = context.WithValue(ctx, logging.RequestIDKey, requestID)

	slog.InfoContext(ctx, "scheduling_service_invoked",
		"request_id", requestID,
		"event_size", len(event),
	)

	defer func() {
		slog.InfoContext(ctx, "invocation_complete",
			"request_id", requestID,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}()

	// 1. Config
	supaProj := os.Getenv("SUPABASE_PROJECT_ID")
	supaKey := os.Getenv("SUPABASE_KEY")
	appAuth := os.Getenv("APPFOLIO_AUTH_HEADER")
	appDevID := os.Getenv("APPFOLIO_DEVELOPER_ID")
	searchURL := os.Getenv("SEARCH_SERVICE_URL")
	openaiKey := os.Getenv("OPENAI_API_KEY")

	if supaProj == "" || supaKey == "" || appAuth == "" || appDevID == "" || searchURL == "" {
		slog.ErrorContext(ctx, "missing_env_vars",
			"request_id", requestID,
			"supabase_project", supaProj != "",
			"supabase_key", supaKey != "",
			"appfolio_auth", appAuth != "",
			"appfolio_dev_id", appDevID != "",
			"search_url", searchURL != "",
		)
		return errorResponse(500, "Missing configuration"), nil
	}

	// 2. Parse Event - handle VAPI tool-calls or regular formats
	var req models.Request
	var extractedPropertyID string

	var genericEvent GenericEvent
	if err := json.Unmarshal(event, &genericEvent); err != nil {
		slog.ErrorContext(ctx, "event_parse_failed", "request_id", requestID, "error", err)
		return errorResponse(400, "Invalid event format"), nil
	}

	bodyToParse := event
	if genericEvent.Body != "" {
		bodyToParse = []byte(genericEvent.Body)
	}

	// Try to detect VAPI tool-calls payload
	var vapiPayload models.VAPIWebhookPayload
	if err := json.Unmarshal(bodyToParse, &vapiPayload); err == nil && vapiPayload.Message.Type == "tool-calls" {
		slog.InfoContext(ctx, "event_type_detected", "request_id", requestID, "type", "vapi_tool_calls")

		// Extract Query from toolCalls
		if len(vapiPayload.Message.ToolCalls) > 0 {
			req.Query = vapiPayload.Message.ToolCalls[0].Function.Arguments.Query
			req.Phone = vapiPayload.Message.ToolCalls[0].Function.Arguments.Phone
			slog.InfoContext(ctx, "vapi_params_extracted", "request_id", requestID, "query", req.Query, "phone", req.Phone)
		}

		// Collect address candidates from tool_call_result messages
		var candidates []clients.AddressCandidate
		for _, msg := range vapiPayload.Message.Artifact.Messages {
			if msg.Role == "tool_call_result" && msg.Result != nil {
				for i, result := range msg.Result.Results {
					if result.Metadata.Address1 != "" && result.Metadata.PropertyId != "" {
						candidates = append(candidates, clients.AddressCandidate{
							Index:      i,
							Address1:   result.Metadata.Address1,
							PropertyId: result.Metadata.PropertyId,
						})
					}
				}
			}
		}

		// Use OpenAI to match query to address if candidates exist
		if len(candidates) > 0 && openaiKey != "" && req.Query != "" {
			slog.InfoContext(ctx, "openai_matching_started", "request_id", requestID, "candidate_count", len(candidates))
			openaiClient := clients.NewOpenAIClient(openaiKey)
			matchedID, err := openaiClient.MatchAddressToQuery(ctx, req.Query, candidates)
			if err != nil {
				slog.WarnContext(ctx, "openai_matching_failed", "request_id", requestID, "error", err)
			} else {
				extractedPropertyID = matchedID
				slog.InfoContext(ctx, "openai_matching_succeeded", "request_id", requestID, "property_id", extractedPropertyID)
			}
		}
	} else if genericEvent.Body != "" {
		slog.InfoContext(ctx, "event_type_detected", "request_id", requestID, "type", "api_gateway")
		if err := json.Unmarshal([]byte(genericEvent.Body), &req); err != nil {
			slog.ErrorContext(ctx, "body_parse_failed", "request_id", requestID, "error", err)
			return errorResponse(400, "Invalid JSON in body"), nil
		}
	} else if genericEvent.Query != "" {
		slog.InfoContext(ctx, "event_type_detected", "request_id", requestID, "type", "direct_invocation")
		req.Query = genericEvent.Query
		req.Phone = genericEvent.Phone
	} else {
		slog.InfoContext(ctx, "event_type_detected", "request_id", requestID, "type", "raw_request")
		if err := json.Unmarshal(event, &req); err != nil {
			slog.ErrorContext(ctx, "request_parse_failed", "request_id", requestID, "error", err)
			return errorResponse(400, "Invalid request format"), nil
		}
	}

	slog.InfoContext(ctx, "request_parsed", "request_id", requestID, "query", req.Query)

	if req.Query == "" {
		return errorResponse(400, "Query is required"), nil
	}

	// 3. Init Clients
	searchClient := clients.NewSearchClient(searchURL)
	appClient := clients.NewAppFolioClient(appAuth, appDevID)
	supaClient := clients.NewSupabaseClient(supaProj, supaKey)
	calClient := clients.NewCalendarClient()

	// 4. Find Property ID (use OpenAI-matched ID if available)
	var propID string
	if extractedPropertyID != "" {
		slog.InfoContext(ctx, "property_source", "request_id", requestID, "source", "openai", "property_id", extractedPropertyID)
		propID = extractedPropertyID
	} else {
		var err error
		propID, err = searchClient.FindPropertyID(ctx, req.Query)
		if err != nil {
			slog.WarnContext(ctx, "search_failed", "request_id", requestID, "error", err, "query", req.Query)
			return successResponse(models.Response{
				Success:      false,
				Message:      "Could not find property matching query.",
				FormattedMsg: fmt.Sprintf("I couldn't find a property matching '%s'. Could you verify the address?", req.Query),
			}), nil
		}
	}
	slog.InfoContext(ctx, "property_found", "request_id", requestID, "property_id", propID)

	// 5. Fetch Property Details
	prop, err := appClient.GetProperty(ctx, propID)
	if err != nil {
		slog.ErrorContext(ctx, "appfolio_property_failed", "request_id", requestID, "error", err, "property_id", propID)
		return successResponse(models.Response{
			Success:      false,
			Message:      "Property found but details unavailable.",
			FormattedMsg: "I found the property but couldn't access its details right now.",
		}), nil
	}

	// 6. Fetch Property Groups (to find Agent)
	groups, err := appClient.GetPropertyGroups(ctx, prop.PropertyGroupIds)
	if err != nil {
		slog.ErrorContext(ctx, "appfolio_groups_failed", "request_id", requestID, "error", err)
		return successResponse(models.Response{
			Success:      false,
			Property:     mapPropertyInfo(prop),
			Message:      "Could not determine agent.",
			FormattedMsg: fmt.Sprintf("I have the details for %s, but I'm having trouble finding the assigned agent.", prop.Address1),
		}), nil
	}

	// 7. Map Agent
	agent := logic.MapAgent(groups)
	if agent == nil {
		slog.WarnContext(ctx, "agent_mapping_failed", "request_id", requestID)
		return successResponse(models.Response{
			Success:      false,
			Property:     mapPropertyInfo(prop),
			Message:      "No leasing agent assigned (No PD group).",
			FormattedMsg: fmt.Sprintf("I checked %s, but there doesn't seem to be a leasing agent assigned to it yet.", prop.Address1),
		}), nil
	}
	slog.InfoContext(ctx, "agent_mapped", "request_id", requestID, "name", agent.Name, "email", agent.Email, "zone", agent.Zone)

	// 8. Get Calendar Access Token
	token, err := supaClient.GetAccessToken(ctx, agent.Email)
	if err != nil {
		slog.ErrorContext(ctx, "token_fetch_failed", "request_id", requestID, "email", agent.Email, "error", err)
		return successResponse(models.Response{
			Success:      false,
			Property:     mapPropertyInfo(prop),
			Agent:        *agent,
			Message:      "Agent calendar access unavailable.",
			FormattedMsg: fmt.Sprintf("I'd love to schedule a viewing for %s, but I can't access %s's calendar right now. Please email them at %s.", prop.Address1, agent.Name, agent.Email),
		}), nil
	}

	// 9. Get Busy Slots (in PST)
	pstLoc, _ := time.LoadLocation("America/Los_Angeles")
	now := time.Now().In(pstLoc)
	timeMax := now.AddDate(0, 0, 7)
	busySlots, err := calClient.GetBusySlots(ctx, token, agent.Email, now, timeMax)
	if err != nil {
		slog.ErrorContext(ctx, "calendar_fetch_failed", "request_id", requestID, "error", err)
		return successResponse(models.Response{
			Success:      false,
			Property:     mapPropertyInfo(prop),
			Agent:        *agent,
			Message:      "Failed to read calendar.",
			FormattedMsg: fmt.Sprintf("I'm having trouble checking %s's availability. Please contact them directly at %s.", agent.Name, agent.Email),
		}), nil
	}

	// 10. Generate Availability
	availableSlots, daysChecked, totalSlots := logic.GenerateAvailableSlots(busySlots, now)

	// 11. Format Message
	avail := models.Availability{
		TotalSlotsAvailable: len(availableSlots),
		DaysChecked:         daysChecked,
		Slots:               limitSlots(availableSlots, 30),
	}

	formattedMsg := formatMessage(mapPropertyInfo(prop), *agent, avail, totalSlots)

	slog.InfoContext(ctx, "scheduling_success",
		"request_id", requestID,
		"property_id", propID,
		"agent", agent.Name,
		"slots_available", len(availableSlots),
		"days_checked", daysChecked,
	)

	return successResponse(models.Response{
		Success:      true,
		Property:     mapPropertyInfo(prop),
		Agent:        *agent,
		Availability: avail,
		Message:      "Success",
		FormattedMsg: formattedMsg,
	}), nil
}

func mapPropertyInfo(p *models.AppFolioProperty) models.PropertyInfo {
	return models.PropertyInfo{
		ID:      p.ID,
		Name:    p.Name,
		Address: p.Address1,
		City:    p.City,
		State:   p.State,
	}
}

func limitSlots(slots []models.TimeSlot, max int) []models.TimeSlot {
	if len(slots) > max {
		return slots[:max]
	}
	return slots
}

func formatMessage(prop models.PropertyInfo, agent models.AgentInfo, avail models.Availability, totalGenerated int) string {
	msg := fmt.Sprintf("🏠 PROPERTY: %s\n📍 %s, %s, %s\n\n", prop.Name, prop.Address, prop.City, prop.State)
	msg += fmt.Sprintf("👤 LEASING AGENT: %s\n📧 Email: %s\n\n", agent.Name, agent.Email)

	if len(avail.Slots) == 0 {
		msg += fmt.Sprintf("📅 SHOWING AVAILABILITY:\nNo available time slots found in the next %d days.\n", avail.DaysChecked)
		msg += fmt.Sprintf("%s's calendar is fully booked.\n\n", agent.Name)
		msg += fmt.Sprintf("📞 Please contact %s directly at %s to schedule.", agent.Name, agent.Email)
		return msg
	}

	msg += "📅 AVAILABLE SHOWING TIMES:\n\n"

	// Group by date
	slotsByDate := make(map[string][]string)
	var orderedDates []string

	for _, slot := range avail.Slots {
		if _, exists := slotsByDate[slot.Date]; !exists {
			orderedDates = append(orderedDates, slot.Date)
		}
		slotsByDate[slot.Date] = append(slotsByDate[slot.Date], slot.Time)
	}

	// Show first 5 days
	count := 0
	for _, date := range orderedDates {
		if count >= 5 {
			break
		}
		times := slotsByDate[date]
		msg += fmt.Sprintf("%s:\n", date)

		// Show first 6 times
		for i, t := range times {
			if i >= 6 {
				msg += fmt.Sprintf("  • ...%d more times available\n", len(times)-6)
				break
			}
			msg += fmt.Sprintf("  • %s\n", t)
		}
		msg += "\n"
		count++
	}

	if len(orderedDates) > 5 {
		msg += fmt.Sprintf("...and %d more days with availability\n", len(orderedDates)-5)
	}

	msg += fmt.Sprintf("\n📞 Contact %s at %s to schedule your showing.", agent.Name, agent.Email)
	return msg
}

func errorResponse(status int, msg string) LambdaResponse {
	body, _ := json.Marshal(map[string]string{"error": msg})
	return LambdaResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}
}

func successResponse(resp models.Response) LambdaResponse {
	body, _ := json.Marshal(resp)
	return LambdaResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}
}

func main() {
	lambda.Start(HandleRequest)
}
