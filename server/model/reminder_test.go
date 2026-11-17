package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeReminders(t *testing.T) {
	got, err := NormalizeReminders([]int{1440, 5, 5, 0})
	require.NoError(t, err)
	assert.Equal(t, []int{0, 5, 1440}, got)

	got, err = NormalizeReminders([]int{})
	require.NoError(t, err)
	assert.Nil(t, got)

	_, err = NormalizeReminders([]int{7})
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestEventNextStart(t *testing.T) {
	ms := func(y int, m time.Month, d, h int) int64 {
		return time.Date(y, m, d, h, 0, 0, 0, time.UTC).UnixMilli()
	}
	single := &Event{StartAt: ms(2026, 1, 5, 9)}
	next, ok := single.NextStart(ms(2026, 1, 5, 8))
	assert.True(t, ok)
	assert.Equal(t, single.StartAt, next)
	_, ok = single.NextStart(single.StartAt)
	assert.False(t, ok)

	weekly := &Event{StartAt: ms(2026, 1, 5, 9), TimeZone: "UTC", Recurrence: &EventRecurrence{Frequency: EventRecurrenceWeekly, Interval: 1, Weekdays: []int{1, 3}}}
	next, ok = weekly.NextStart(ms(2026, 1, 5, 9)) // Monday -> Wednesday
	assert.True(t, ok)
	assert.Equal(t, ms(2026, 1, 7, 9), next)

	monthly := &Event{StartAt: ms(2026, 1, 31, 9), TimeZone: "UTC", Recurrence: &EventRecurrence{Frequency: EventRecurrenceMonthly, Interval: 1}}
	next, _ = monthly.NextStart(ms(2026, 1, 31, 9))
	assert.Equal(t, ms(2026, 2, 28, 9), next)

	tokyo := &Event{StartAt: ms(2026, 1, 5, 0), TimeZone: "Asia/Tokyo", Recurrence: &EventRecurrence{Frequency: EventRecurrenceDaily, Interval: 2}}
	next, _ = tokyo.NextStart(tokyo.StartAt)
	assert.Equal(t, ms(2026, 1, 7, 0), next)

	ended := &Event{StartAt: ms(2026, 1, 5, 9), TimeZone: "UTC", Recurrence: &EventRecurrence{Frequency: EventRecurrenceDaily, Interval: 1, Until: "2026-01-06"}}
	_, ok = ended.NextStart(ms(2026, 1, 6, 9))
	assert.False(t, ok)
}

func TestEventNextStartWithOverrides(t *testing.T) {
	ms := func(y int, m time.Month, d, h int) int64 {
		return time.Date(y, m, d, h, 0, 0, 0, time.UTC).UnixMilli()
	}
	// Weekly on Wednesdays 10:00 starting Jan 7, 2026.
	event := &Event{StartAt: ms(2026, 1, 7, 10), TimeZone: "UTC", Recurrence: &EventRecurrence{Frequency: EventRecurrenceWeekly, Interval: 1, Weekdays: []int{3}}}
	slot := func(day int) int64 { return ms(2026, 1, day, 10) }

	// Moving the Jan 14 occurrence to Tuesday 9:00 makes it the next start.
	movedFrom := slot(14)
	movedTo := ms(2026, 1, 13, 9)
	overrides, err := event.NormalizeEventOverride(movedFrom, EventOverride{StartAt: movedTo, EndAt: movedTo + 3600_000})
	require.NoError(t, err)
	event.Overrides = overrides
	next, ok := event.NextStart(ms(2026, 1, 10, 0))
	require.True(t, ok)
	assert.Equal(t, movedTo, next)
	// The following occurrence is the untouched slot a week later.
	next, _ = event.NextStart(movedTo)
	assert.Equal(t, slot(21), next)

	// Cancelling removes the occurrence from the sequence.
	cancelled := &Event{StartAt: event.StartAt, TimeZone: "UTC", Recurrence: event.Recurrence.Clone()}
	cancelledOverrides, err := cancelled.NormalizeEventOverride(movedFrom, EventOverride{Cancelled: true})
	require.NoError(t, err)
	cancelled.Overrides = cancelledOverrides
	next, ok = cancelled.NextStart(ms(2026, 1, 10, 0))
	require.True(t, ok)
	assert.Equal(t, slot(21), next)

	// An occurrence moved into the future from a slot before `after` still counts.
	pulled := &Event{StartAt: event.StartAt, TimeZone: "UTC", Recurrence: event.Recurrence.Clone()}
	farSlot := ms(2026, 2, 4, 10) // a Wednesday three weeks out
	pulledOverrides, err := pulled.NormalizeEventOverride(farSlot, EventOverride{StartAt: ms(2026, 1, 9, 8), EndAt: ms(2026, 1, 9, 9)})
	require.NoError(t, err)
	pulled.Overrides = pulledOverrides
	next, ok = pulled.NextStart(ms(2026, 1, 8, 0))
	require.True(t, ok)
	assert.Equal(t, ms(2026, 1, 9, 8), next)
}
