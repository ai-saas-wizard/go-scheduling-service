package logic

import (
	"strings"

	"github.com/vishnuanilkumar/go-scheduling-service/internal/models"
)

var PDAgentMap = map[string]models.AgentInfo{
	"PD1": {ID: "59a26c67-5791-11f0-b6c3-02094d1ce055", Name: "Gracie", Email: "gracie@ltrealestateco.com", Zone: "PD1"},
	"PD2": {ID: "dcb80b8a-66bd-11ee-b6c3-02094d1ce055", Name: "Elizabeth", Email: "elizabeth@ltrealestateco.com", Zone: "PD2"},
	"PD3": {ID: "4d6b75fd-5791-11f0-b6c3-02094d1ce055", Name: "Alexandra", Email: "alexandra@ltrealestateco.com", Zone: "PD3"},
	"PD4": {ID: "4b8f5454-ef30-11ef-b6c3-02094d1ce055", Name: "Brandi", Email: "brandi@ltrealestateco.com", Zone: "PD4"},
}

// MapAgent finds the agent based on property group names (looking for PD1, PD2, etc.)
func MapAgent(groups []models.AppFolioGroup) *models.AgentInfo {
	for _, group := range groups {
		name := strings.ToUpper(strings.TrimSpace(group.Name))
		if agent, ok := PDAgentMap[name]; ok {
			agent.ZoneGroup = group.Name
			return &agent
		}
	}
	return nil
}
