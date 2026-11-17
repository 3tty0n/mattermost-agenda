package model

import (
	"cmp"
	"fmt"
	"slices"
	"time"
)

// MaxEventOverrides caps the exceptions stored on one recurring event.
const MaxEventOverrides = 500

// EventOverride moves or cancels one occurrence of a recurring event. OriginalStart is the exact
// start instant the occurrence would have had in the unmodified series; it identifies the
// occurrence. Cancelled removes it; otherwise StartAt and EndAt replace its times. Everything else
// (title, assignees, reminders…) stays shared with the series.
type EventOverride struct {
	OriginalStart int64 `json:"originalStart"`
	StartAt       int64 `json:"startAt,omitempty"`
	EndAt         int64 `json:"endAt,omitempty"`
	Cancelled     bool  `json:"cancelled,omitempty"`
}

// EffectiveOverride returns the override that replaces or removes the occurrence starting at start.
func (e *Event) EffectiveOverride(start int64) (EventOverride, bool) {
	for _, override := range e.Overrides {
		if override.OriginalStart == start {
			return override, true
		}
	}
	return EventOverride{}, false
}

// occurrenceStartAt reports whether start is the exact start instant of a series occurrence.
// It mirrors the slot math in NextStart, so an override can only name a real occurrence.
func (e *Event) occurrenceStartAt(start int64) bool {
	if e.Recurrence.IsZero() || start <= 0 || start > maxEventAt {
		return false
	}
	loc, anchorTime := e.recurrenceLocation()
	target := utcMidnight(time.UnixMilli(start).In(loc))
	anchor := utcMidnight(anchorTime)
	for day, i := anchor, 0; !day.After(target) && i < 36600; day, i = day.AddDate(0, 0, 1), i+1 {
		if e.Recurrence.Until != "" && day.Format("2006-01-02") > e.Recurrence.Until {
			return false
		}
		if e.Recurrence.matches(day, anchor) && e.slotStart(day, anchorTime, loc) == start {
			return true
		}
	}
	return false
}

// NormalizeEventOverride validates a change to one occurrence of the event and returns the updated
// overrides sorted by original start. A cancelled override clears the moved times; a moved one
// must carry a valid interval in the event's all-day convention.
func (e *Event) NormalizeEventOverride(originalStart int64, override EventOverride) ([]EventOverride, error) {
	if e.Recurrence.IsZero() {
		return nil, fmt.Errorf("%w: event does not repeat", ErrInvalid)
	}
	if originalStart <= 0 || originalStart > maxEventAt {
		return nil, fmt.Errorf("%w: invalid occurrence", ErrInvalid)
	}
	if !e.occurrenceStartAt(originalStart) {
		return nil, fmt.Errorf("%w: not an occurrence of this series", ErrInvalid)
	}
	if override.Cancelled {
		override.StartAt, override.EndAt = 0, 0
	} else {
		startAt, endAt, err := NormalizeEventTimes(override.StartAt, override.EndAt, e.AllDay)
		if err != nil {
			return nil, err
		}
		override.StartAt, override.EndAt = startAt, endAt
	}
	out := make([]EventOverride, 0, len(e.Overrides)+1)
	for _, existing := range e.Overrides {
		if existing.OriginalStart != originalStart {
			out = append(out, existing)
		}
	}
	out = append(out, EventOverride{
		OriginalStart: originalStart,
		StartAt:       override.StartAt,
		EndAt:         override.EndAt,
		Cancelled:     override.Cancelled,
	})
	slices.SortFunc(out, func(a, b EventOverride) int {
		return cmp.Compare(a.OriginalStart, b.OriginalStart)
	})
	if len(out) > MaxEventOverrides {
		return nil, fmt.Errorf("%w: too many occurrence overrides", ErrInvalid)
	}
	return out, nil
}

// PruneEventOverrides drops overrides that no longer name a real occurrence of the series, which
// happens when the series is re-anchored, and returns the survivors in their stored order.
func (e *Event) PruneEventOverrides() []EventOverride {
	if len(e.Overrides) == 0 {
		return nil
	}
	latest := e.Overrides[0].OriginalStart
	for _, override := range e.Overrides {
		latest = max(latest, override.OriginalStart)
	}
	loc, anchorTime := e.recurrenceLocation()
	anchor := utcMidnight(anchorTime)
	limit := utcMidnight(time.UnixMilli(latest).In(loc))
	slots := make(map[int64]struct{})
	for day, i := anchor, 0; !day.After(limit) && i < 36600; day, i = day.AddDate(0, 0, 1), i+1 {
		if e.Recurrence.Until != "" && day.Format("2006-01-02") > e.Recurrence.Until {
			break
		}
		if e.Recurrence.matches(day, anchor) {
			slots[e.slotStart(day, anchorTime, loc)] = struct{}{}
		}
	}
	out := make([]EventOverride, 0, len(e.Overrides))
	for _, override := range e.Overrides {
		if _, ok := slots[override.OriginalStart]; ok {
			out = append(out, override)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// recurrenceLocation resolves the timezone the slot math runs in: UTC for all-day events, the
// event's zone for timed ones (falling back to UTC when it cannot be loaded).
func (e *Event) recurrenceLocation() (*time.Location, time.Time) {
	loc := time.UTC
	if !e.AllDay {
		if loaded, err := time.LoadLocation(e.TimeZone); err == nil {
			loc = loaded
		}
	}
	return loc, time.UnixMilli(e.StartAt).In(loc)
}

// slotStart is the start instant of the occurrence on the given civil day.
func (e *Event) slotStart(day time.Time, anchorTime time.Time, loc *time.Location) int64 {
	if e.AllDay {
		return day.UnixMilli()
	}
	return time.Date(day.Year(), day.Month(), day.Day(), anchorTime.Hour(), anchorTime.Minute(), 0, 0, loc).UnixMilli()
}

// EventOverridesEqual compares two override lists including their order.
func EventOverridesEqual(a, b []EventOverride) bool {
	if len(a) != len(b) {
		return false
	}
	return slices.Equal(a, b)
}
