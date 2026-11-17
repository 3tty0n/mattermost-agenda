package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCalendarColors(t *testing.T) {
	t.Run("empty colors stay empty", func(t *testing.T) {
		got, err := NormalizeCalendarColors(CalendarColors{})
		require.NoError(t, err)
		assert.Equal(t, CalendarColors{}, got)
	})

	t.Run("hex colors are lowercased and kept", func(t *testing.T) {
		got, err := NormalizeCalendarColors(CalendarColors{Mine: " #4F7BD9 ", Team: "#62a461"})
		require.NoError(t, err)
		assert.Equal(t, CalendarColors{Mine: "#4f7bd9", Team: "#62a461"}, got)
	})

	for _, value := range []string{"red", "#12345", "#1234567", "#12345g", "4f7bd9", "#12345 "} {
		t.Run("rejects "+value, func(t *testing.T) {
			_, err := NormalizeCalendarColors(CalendarColors{Team: value})
			assert.ErrorIs(t, err, ErrInvalid)
			assert.NotErrorIs(t, err, errors.New("unrelated"))
		})
	}
}
