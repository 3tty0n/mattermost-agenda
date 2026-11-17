package kvstore

import (
	taskmodel "github.com/3tty0n/mattermost-tasks/server/model"
)

// userPrefsRecord is the per-user preference blob stored under userprefs:<userID>. It is a single
// struct so later preferences can be added without a storage migration.
type userPrefsRecord struct {
	CalendarColors *taskmodel.CalendarColors `json:"calendarColors,omitempty"`
}

func userPrefsKey(userID string) string {
	return "userprefs:" + userID
}

// UserPrefsStore persists per-user preferences. A user's record is written only by that user, so
// the setters need no compare-and-swap.
type UserPrefsStore interface {
	GetUserCalendarColors(userID string) (taskmodel.CalendarColors, error)
	SetUserCalendarColors(userID string, colors taskmodel.CalendarColors) error
}

var _ UserPrefsStore = (*KVStore)(nil)

// GetUserCalendarColors returns the user's calendar colors; a missing record is the zero value.
func (s *KVStore) GetUserCalendarColors(userID string) (taskmodel.CalendarColors, error) {
	var record userPrefsRecord
	if err := s.kv.Get(userPrefsKey(userID), &record); err != nil {
		return taskmodel.CalendarColors{}, err
	}
	if record.CalendarColors == nil {
		return taskmodel.CalendarColors{}, nil
	}
	return *record.CalendarColors, nil
}

// SetUserCalendarColors replaces the user's calendar colors, preserving any other preferences in
// the same record.
func (s *KVStore) SetUserCalendarColors(userID string, colors taskmodel.CalendarColors) error {
	var record userPrefsRecord
	if err := s.kv.Get(userPrefsKey(userID), &record); err != nil {
		return err
	}
	if colors == (taskmodel.CalendarColors{}) {
		record.CalendarColors = nil
	} else {
		record.CalendarColors = &colors
	}
	_, err := s.kv.Set(userPrefsKey(userID), record)
	return err
}
