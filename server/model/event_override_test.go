package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func weeklyEvent() *Event {
	return &Event{
		StartAt:  time.Date(2026, time.January, 7, 10, 0, 0, 0, time.UTC).UnixMilli(), // Wednesday
		TimeZone: "UTC",
		Recurrence: &EventRecurrence{
			Frequency: EventRecurrenceWeekly,
			Interval:  1,
			Weekdays:  []int{3},
		},
	}
}

func TestNormalizeEventOverride(t *testing.T) {
	event := weeklyEvent()

	// Moving the second occurrence (Jan 14) to Tuesday Jan 13.
	original := time.Date(2026, time.January, 14, 10, 0, 0, 0, time.UTC).UnixMilli()
	movedStart := time.Date(2026, time.January, 13, 9, 0, 0, 0, time.UTC).UnixMilli()
	movedEnd := time.Date(2026, time.January, 13, 10, 0, 0, 0, time.UTC).UnixMilli()
	overrides, err := event.NormalizeEventOverride(original, EventOverride{StartAt: movedStart, EndAt: movedEnd})
	require.NoError(t, err)
	assert.Equal(t, []EventOverride{{OriginalStart: original, StartAt: movedStart, EndAt: movedEnd}}, overrides)

	// Re-moving the same occurrence replaces the earlier override.
	overrides, err = event.NormalizeEventOverride(original, EventOverride{StartAt: movedEnd, EndAt: movedEnd + 3600_000})
	require.NoError(t, err)
	require.Len(t, overrides, 1)
	assert.Equal(t, movedEnd, overrides[0].StartAt)

	// Cancelling drops the moved times and keeps one entry per occurrence.
	overrides, err = event.NormalizeEventOverride(original, EventOverride{Cancelled: true})
	require.NoError(t, err)
	require.Len(t, overrides, 1)
	assert.True(t, overrides[0].Cancelled)
	assert.Zero(t, overrides[0].StartAt)
	assert.Zero(t, overrides[0].EndAt)
	event.Overrides = overrides

	// A different occurrence coexists with the first override.
	other := time.Date(2026, time.January, 21, 10, 0, 0, 0, time.UTC).UnixMilli()
	overrides, err = event.NormalizeEventOverride(other, EventOverride{StartAt: movedStart, EndAt: movedEnd})
	require.NoError(t, err)
	require.Len(t, overrides, 2)
	assert.Equal(t, original, overrides[0].OriginalStart) // sorted by original start
	assert.Equal(t, other, overrides[1].OriginalStart)
}

func TestNormalizeEventOverrideValidation(t *testing.T) {
	event := weeklyEvent()
	validStart := time.Date(2026, time.January, 14, 10, 0, 0, 0, time.UTC).UnixMilli()
	movedStart := time.Date(2026, time.January, 13, 9, 0, 0, 0, time.UTC).UnixMilli()
	movedEnd := time.Date(2026, time.January, 13, 10, 0, 0, 0, time.UTC).UnixMilli()

	t.Run("not a slot", func(t *testing.T) {
		_, err := event.NormalizeEventOverride(movedStart, EventOverride{StartAt: movedStart, EndAt: movedEnd})
		assert.ErrorIs(t, err, ErrInvalid)
	})
	t.Run("non-recurring event", func(t *testing.T) {
		_, err := (&Event{StartAt: validStart}).NormalizeEventOverride(validStart, EventOverride{Cancelled: true})
		assert.ErrorIs(t, err, ErrInvalid)
	})
	t.Run("end before start", func(t *testing.T) {
		_, err := event.NormalizeEventOverride(validStart, EventOverride{StartAt: movedEnd, EndAt: movedStart})
		assert.ErrorIs(t, err, ErrInvalid)
	})
	t.Run("missing times when moving", func(t *testing.T) {
		_, err := event.NormalizeEventOverride(validStart, EventOverride{})
		assert.ErrorIs(t, err, ErrInvalid)
	})
	t.Run("all-day slots are UTC midnights", func(t *testing.T) {
		allDay := weeklyEvent()
		allDay.AllDay = true
		_, err := allDay.NormalizeEventOverride(validStart, EventOverride{Cancelled: true})
		assert.ErrorIs(t, err, ErrInvalid)

		midnight := time.Date(2026, time.January, 14, 0, 0, 0, 0, time.UTC).UnixMilli()
		overrides, err := allDay.NormalizeEventOverride(midnight, EventOverride{
			StartAt: time.Date(2026, time.January, 13, 0, 0, 0, 0, time.UTC).UnixMilli(),
			EndAt:   time.Date(2026, time.January, 14, 0, 0, 0, 0, time.UTC).UnixMilli(),
		})
		require.NoError(t, err)
		require.Len(t, overrides, 1)
	})
}

func TestPruneEventOverrides(t *testing.T) {
	event := weeklyEvent()
	kept := time.Date(2026, time.January, 14, 10, 0, 0, 0, time.UTC).UnixMilli()
	dropped := time.Date(2026, time.January, 21, 10, 0, 0, 0, time.UTC).UnixMilli()
	overrides, err := event.NormalizeEventOverride(kept, EventOverride{Cancelled: true})
	require.NoError(t, err)
	event.Overrides = overrides
	overrides, err = event.NormalizeEventOverride(dropped, EventOverride{Cancelled: true})
	require.NoError(t, err)
	event.Overrides = overrides

	// Re-anchoring the series to Tuesdays invalidates both slots.
	event.StartAt = time.Date(2026, time.January, 6, 10, 0, 0, 0, time.UTC).UnixMilli()
	event.Recurrence.Weekdays = []int{2}
	assert.Nil(t, event.PruneEventOverrides())

	// A matching anchor keeps the overrides.
	event.StartAt = time.Date(2026, time.January, 7, 10, 0, 0, 0, time.UTC).UnixMilli()
	event.Recurrence.Weekdays = []int{3}
	survivors := event.PruneEventOverrides()
	require.Len(t, survivors, 2)
	assert.Equal(t, kept, survivors[0].OriginalStart)
	assert.Equal(t, dropped, survivors[1].OriginalStart)
}
