package logic

import (
	"log/slog"
	"time"

	"github.com/vishnuanilkumar/go-scheduling-service/internal/models"
)

const (
	WorkStartHour = 9
	WorkEndHour   = 17
	SlotDuration  = 30 * time.Minute
	MaxDays       = 7
)

// GenerateAvailableSlots calculates free slots given busy periods
func GenerateAvailableSlots(busySlots []models.TimeRange, referenceTime time.Time) ([]models.TimeSlot, int, int) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		loc = time.UTC
		slog.Warn("timezone_load_failed", "timezone", "America/Los_Angeles", "error", err)
	}

	// Calculate the minimum start time (2 hours from reference time)
	minStartTime := referenceTime.Add(2 * time.Hour)

	// Normalize search start
	startSearch := referenceTime.In(loc)

	var availableSlots []models.TimeSlot
	daysChecked := 0
	totalSlots := 0

	for d := 0; d < MaxDays; d++ {
		dayDate := startSearch.AddDate(0, 0, d)

		// Skip weekends
		if dayDate.Weekday() == time.Saturday || dayDate.Weekday() == time.Sunday {
			continue
		}
		daysChecked++

		// Set work hours for this day
		workStart := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(), WorkStartHour, 0, 0, 0, loc)
		workEnd := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(), WorkEndHour, 0, 0, 0, loc)

		// Friday check: Ends at 3:30 PM (15:30)
		if dayDate.Weekday() == time.Friday {
			workEnd = time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(), 15, 30, 0, 0, loc)
		}

		// Adjust workStart if it's before minStartTime (ensure 2h buffer)
		if workStart.Before(minStartTime) {
			workStart = minStartTime

			if workStart.After(workEnd) {
				continue
			}

			// Round up to next 30 min slot
			remainder := workStart.Minute() % 30
			add := 0
			if remainder != 0 {
				add = 30 - remainder
			}
			workStart = workStart.Add(time.Duration(add) * time.Minute)
			workStart = workStart.Truncate(time.Minute)
		}

		// Generate slots
		curr := workStart
		for curr.Add(SlotDuration).Before(workEnd) || curr.Add(SlotDuration).Equal(workEnd) {
			slotEnd := curr.Add(SlotDuration)

			if !isBusy(curr, slotEnd, busySlots) {
				availableSlots = append(availableSlots, formatSlot(curr, slotEnd))
			}
			totalSlots++

			curr = slotEnd
		}
	}

	return availableSlots, daysChecked, totalSlots
}

func isBusy(start, end time.Time, busy []models.TimeRange) bool {
	loc := start.Location()

	for _, b := range busy {
		busyStart := b.Start.In(loc)
		busyEnd := b.End.In(loc)

		if start.Before(busyEnd) && end.After(busyStart) {
			return true
		}
	}
	return false
}

func formatSlot(start, end time.Time) models.TimeSlot {
	return models.TimeSlot{
		Date:  start.Format("Monday, January 2, 2006"),
		Time:  start.Format("3:04 PM"),
		Start: start,
		End:   end,
	}
}
