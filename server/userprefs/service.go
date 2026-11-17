// Package userprefs owns the per-user preference use cases. Preferences are keyed by the calling
// user, so a preference may only ever be read or written for the caller themself.
package userprefs

import (
	"github.com/3tty0n/mattermost-tasks/server/model"
	"github.com/3tty0n/mattermost-tasks/server/store/kvstore"
)

// Service implements per-user preference use cases.
type Service struct {
	store kvstore.UserPrefsStore
}

// NewService builds a Service on top of the plugin KV store.
func NewService(store kvstore.UserPrefsStore) *Service {
	return &Service{store: store}
}

// CalendarColors returns the user's calendar colors; a user without preferences gets the defaults.
func (s *Service) CalendarColors(userID string) (model.CalendarColors, error) {
	return s.store.GetUserCalendarColors(userID)
}

// SetCalendarColors validates and stores the user's calendar colors and returns the stored form.
func (s *Service) SetCalendarColors(userID string, colors model.CalendarColors) (model.CalendarColors, error) {
	normalized, err := model.NormalizeCalendarColors(colors)
	if err != nil {
		return model.CalendarColors{}, err
	}
	if err := s.store.SetUserCalendarColors(userID, normalized); err != nil {
		return model.CalendarColors{}, err
	}
	return normalized, nil
}
