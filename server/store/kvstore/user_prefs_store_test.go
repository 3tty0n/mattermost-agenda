package kvstore

import (
	"testing"

	taskmodel "github.com/3tty0n/mattermost-tasks/server/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserCalendarColors(t *testing.T) {
	s := NewStoreWithKV(NewMemoryKV())
	userID, otherID := "user-1", "user-2"

	colors, err := s.GetUserCalendarColors(userID)
	require.NoError(t, err)
	assert.Equal(t, taskmodel.CalendarColors{}, colors)

	err = s.SetUserCalendarColors(userID, taskmodel.CalendarColors{Mine: "#4f7bd9", Team: "#62a461"})
	require.NoError(t, err)

	colors, err = s.GetUserCalendarColors(userID)
	require.NoError(t, err)
	assert.Equal(t, taskmodel.CalendarColors{Mine: "#4f7bd9", Team: "#62a461"}, colors)

	// Preferences are per user and never leak between records.
	other, err := s.GetUserCalendarColors(otherID)
	require.NoError(t, err)
	assert.Equal(t, taskmodel.CalendarColors{}, other)

	// Clearing both colors drops the stored pointer so the record stays compact.
	err = s.SetUserCalendarColors(userID, taskmodel.CalendarColors{})
	require.NoError(t, err)
	colors, err = s.GetUserCalendarColors(userID)
	require.NoError(t, err)
	assert.Equal(t, taskmodel.CalendarColors{}, colors)
}
