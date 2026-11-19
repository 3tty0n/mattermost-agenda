package events

import (
	"strings"
	"testing"
	"time"

	mmodel "github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/3tty0n/mattermost-tasks/server/access"
	taskmodel "github.com/3tty0n/mattermost-tasks/server/model"
	"github.com/3tty0n/mattermost-tasks/server/store/kvstore"
	"github.com/3tty0n/mattermost-tasks/server/testutil"
)

type fakePublisher struct {
	events []*taskmodel.Event
}

func (p *fakePublisher) EventChanged(_ string, event *taskmodel.Event) {
	p.events = append(p.events, event)
}

type testEnv struct {
	service   *EventService
	api       *testutil.API
	store     *kvstore.KVStore
	publisher *fakePublisher
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	api := testutil.NewAPI()
	t.Cleanup(func() { api.AssertExpectations(t) })
	client := pluginapi.NewClient(api, &plugintest.Driver{})
	store := kvstore.NewStoreWithKV(kvstore.NewMemoryKV())
	publisher := &fakePublisher{}
	service := NewEventService(client, store, access.NewChecker(client), publisher, WithEventClock(func() int64 { return 1000 }))
	return &testEnv{service: service, api: api, store: store, publisher: publisher}
}

func allowChannel(api *testutil.API, userID, channelID, teamID string) {
	api.On("GetChannel", channelID).Return(&mmodel.Channel{Id: channelID, TeamId: teamID, Type: mmodel.ChannelTypeOpen}, nil).Maybe()
	api.On("HasPermissionToChannel", userID, channelID, mmodel.PermissionReadChannel).Return(true).Maybe()
}

func allowTeam(api *testutil.API, userID, teamID string) {
	api.On("GetTeam", teamID).Return(&mmodel.Team{Id: teamID}, nil).Maybe()
	api.On("HasPermissionToTeam", userID, teamID, mmodel.PermissionViewTeam).Return(true).Maybe()
}

func denyAdmin(api *testutil.API, userID string) {
	api.On("HasPermissionTo", userID, mmodel.PermissionManageSystem).Return(false).Maybe()
}

func TestEventAllDayNormalization(t *testing.T) {
	env := newTestEnv(t)
	teamID, channelID := mmodel.NewId(), mmodel.NewId()
	creatorID := mmodel.NewId()
	allowChannel(env.api, creatorID, channelID, teamID)
	denyAdmin(env.api, creatorID)

	start := time.Date(2026, time.January, 5, 14, 0, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2026, time.January, 7, 14, 0, 0, 0, time.UTC).UnixMilli()
	event, err := env.service.Create(nil, "/events", creatorID, taskmodel.CreateEventInput{
		ChannelID:  channelID,
		Visibility: taskmodel.TaskVisibilityChannel,
		Title:      "Offsite",
		StartAt:    start,
		EndAt:      end,
		AllDay:     true,
		TimeZone:   "UTC",
	})
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC).UnixMilli(), event.StartAt)
	assert.Equal(t, time.Date(2026, time.January, 7, 0, 0, 0, 0, time.UTC).UnixMilli(), event.EndAt)
	assert.Len(t, env.publisher.events, 1)
}

func TestPrivateEventVisibilityAndRange(t *testing.T) {
	env := newTestEnv(t)
	teamID, channelID := mmodel.NewId(), mmodel.NewId()
	creatorID, otherID := mmodel.NewId(), mmodel.NewId()
	allowTeam(env.api, creatorID, teamID)
	allowTeam(env.api, otherID, teamID)
	allowChannel(env.api, creatorID, channelID, teamID)
	allowChannel(env.api, otherID, channelID, teamID)
	denyAdmin(env.api, creatorID)
	denyAdmin(env.api, otherID)

	start := time.Date(2026, time.January, 5, 9, 0, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2026, time.January, 5, 10, 0, 0, 0, time.UTC).UnixMilli()
	event, err := env.service.Create(nil, "/events", creatorID, taskmodel.CreateEventInput{
		TeamID:     teamID,
		ChannelID:  channelID,
		Visibility: taskmodel.TaskVisibilityPrivate,
		Title:      "Private planning",
		StartAt:    start,
		EndAt:      end,
		TimeZone:   "UTC",
	})
	require.NoError(t, err)
	assert.Empty(t, event.ChannelID)

	_, err = env.service.Get(otherID, event.ID)
	assert.ErrorIs(t, err, taskmodel.ErrNotFound)

	query := taskmodel.EventQuery{
		Scope:   taskmodel.TaskScopeMy,
		From:    start - (24 * 60 * 60 * 1000),
		To:      end + (24 * 60 * 60 * 1000),
		Page:    0,
		PerPage: 20,
	}
	result, err := env.service.List(creatorID, query)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)

	query.Scope = taskmodel.TaskScopeTeam
	query.TeamID = teamID
	// The team view holds shared events only; a private event stays in the Mine view.
	result, err = env.service.List(creatorID, query)
	require.NoError(t, err)
	assert.Zero(t, result.Total)
	result, err = env.service.List(otherID, query)
	require.NoError(t, err)
	assert.Zero(t, result.Total)

	outside := taskmodel.EventQuery{
		Scope:   taskmodel.TaskScopeTeam,
		TeamID:  teamID,
		From:    end + 1,
		To:      end + (24 * 60 * 60 * 1000),
		Page:    0,
		PerPage: 20,
	}
	result, err = env.service.List(creatorID, outside)
	require.NoError(t, err)
	assert.Zero(t, result.Total)
}

func TestRecurringEventIsIncludedOutsideItsBaseRange(t *testing.T) {
	env := newTestEnv(t)
	teamID, channelID := mmodel.NewId(), mmodel.NewId()
	creatorID := mmodel.NewId()
	allowChannel(env.api, creatorID, channelID, teamID)
	allowTeam(env.api, creatorID, teamID)

	start := time.Date(2026, time.January, 5, 9, 0, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2026, time.January, 5, 10, 0, 0, 0, time.UTC).UnixMilli()
	event, err := env.service.Create(nil, "/events", creatorID, taskmodel.CreateEventInput{
		ChannelID:  channelID,
		Visibility: taskmodel.TaskVisibilityChannel,
		Title:      "Standup",
		StartAt:    start,
		EndAt:      end,
		TimeZone:   "UTC",
		Recurrence: &taskmodel.EventRecurrence{
			Frequency: taskmodel.EventRecurrenceWeekly,
			Weekdays:  []int{1, 3},
			Until:     "2026-02-01",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, event.Recurrence)
	assert.Equal(t, []int{1, 3}, event.Recurrence.Weekdays)

	query := taskmodel.EventQuery{
		Scope:   taskmodel.TaskScopeTeam,
		TeamID:  teamID,
		From:    start + (7 * 24 * 60 * 60 * 1000),
		To:      start + (8 * 24 * 60 * 60 * 1000),
		Page:    0,
		PerPage: 20,
	}
	result, err := env.service.List(creatorID, query)
	require.NoError(t, err)
	assert.Zero(t, result.Total)

	query.IncludeRecurring = true
	result, err = env.service.List(creatorID, query)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	require.Len(t, result.Items, 1)
	require.NotNil(t, result.Items[0].Recurrence)
	assert.Equal(t, []int{1, 3}, result.Items[0].Recurrence.Weekdays)
	// The summary carries the overrides so the webapp can expand the series correctly.
	assert.Equal(t, event.Overrides, result.Items[0].Overrides)
}

func TestEventOccurrenceOverride(t *testing.T) {
	env := newTestEnv(t)
	teamID, channelID := mmodel.NewId(), mmodel.NewId()
	creatorID, otherID := mmodel.NewId(), mmodel.NewId()
	allowTeam(env.api, creatorID, teamID)
	allowTeam(env.api, otherID, teamID)
	allowChannel(env.api, creatorID, channelID, teamID)
	allowChannel(env.api, otherID, channelID, teamID)
	denyAdmin(env.api, creatorID)
	denyAdmin(env.api, otherID)

	// Weekly standup on Wednesdays 10:00 UTC.
	start := time.Date(2026, time.January, 7, 10, 0, 0, 0, time.UTC).UnixMilli()
	event, err := env.service.Create(nil, "/events", creatorID, taskmodel.CreateEventInput{
		ChannelID:  channelID,
		Visibility: taskmodel.TaskVisibilityChannel,
		Title:      "Standup",
		StartAt:    start,
		EndAt:      start + 3600_000,
		TimeZone:   "UTC",
		Recurrence: &taskmodel.EventRecurrence{Frequency: taskmodel.EventRecurrenceWeekly, Weekdays: []int{3}},
	})
	require.NoError(t, err)

	second := start + 7*24*3600_000 // Wednesday Jan 14
	movedStart := time.Date(2026, time.January, 13, 9, 0, 0, 0, time.UTC).UnixMilli()
	movedEnd := movedStart + 3600_000

	// A non-creator may not touch the series or a single occurrence.
	title := "Hijack"
	_, err = env.service.Update(nil, "/events", otherID, event.ID, taskmodel.EventPatch{Title: &title})
	assert.ErrorIs(t, err, taskmodel.ErrForbidden)
	_, err = env.service.Update(nil, "/events", otherID, event.ID, taskmodel.EventPatch{
		OccurrenceStartAt: &second,
		StartAt:           &movedStart,
		EndAt:             &movedEnd,
	})
	assert.ErrorIs(t, err, taskmodel.ErrForbidden)

	// Times only: touching any other field in occurrence mode is rejected.
	_, err = env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{
		OccurrenceStartAt: &second,
		StartAt:           &movedStart,
		EndAt:             &movedEnd,
		Title:             &title,
	})
	assert.ErrorIs(t, err, taskmodel.ErrInvalid)
	_, err = env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{OccurrenceStartAt: &second})
	assert.ErrorIs(t, err, taskmodel.ErrInvalid)

	// Moving the second occurrence keeps the series anchor untouched.
	updated, err := env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{
		OccurrenceStartAt: &second,
		StartAt:           &movedStart,
		EndAt:             &movedEnd,
	})
	require.NoError(t, err)
	assert.Equal(t, start, updated.StartAt)
	require.Len(t, updated.Overrides, 1)
	assert.Equal(t, second, updated.Overrides[0].OriginalStart)
	assert.Equal(t, movedStart, updated.Overrides[0].StartAt)

	// The reminder schedule follows the moved occurrence.
	env.publisher.events = nil

	// Re-moving the same occurrence replaces the override, cancelling clears it.
	againStart := movedStart + 1800_000
	againEnd := againStart + 3600_000
	updated, err = env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{
		OccurrenceStartAt: &second,
		StartAt:           &againStart,
		EndAt:             &againEnd,
	})
	require.NoError(t, err)
	require.Len(t, updated.Overrides, 1)
	assert.Equal(t, againStart, updated.Overrides[0].StartAt)

	cancelled := true
	updated, err = env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{
		OccurrenceStartAt:   &second,
		OccurrenceCancelled: &cancelled,
	})
	require.NoError(t, err)
	require.Len(t, updated.Overrides, 1)
	assert.True(t, updated.Overrides[0].Cancelled)
	assert.Zero(t, updated.Overrides[0].StartAt)

	// Occurrence patches are rejected for events that do not repeat.
	once := time.Date(2026, time.January, 20, 10, 0, 0, 0, time.UTC).UnixMilli()
	single, err := env.service.Create(nil, "/events", creatorID, taskmodel.CreateEventInput{
		ChannelID:  channelID,
		Visibility: taskmodel.TaskVisibilityChannel,
		Title:      "Once",
		StartAt:    once,
		EndAt:      once + 3600_000,
		TimeZone:   "UTC",
	})
	require.NoError(t, err)
	_, err = env.service.Update(nil, "/events", creatorID, single.ID, taskmodel.EventPatch{
		OccurrenceStartAt: &once,
		StartAt:           &movedStart,
		EndAt:             &movedEnd,
	})
	assert.ErrorIs(t, err, taskmodel.ErrInvalid)

	// A moved slot outside the series is rejected.
	notASlot := start + 3600_000
	_, err = env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{
		OccurrenceStartAt: &notASlot,
		StartAt:           &movedStart,
		EndAt:             &movedEnd,
	})
	assert.ErrorIs(t, err, taskmodel.ErrInvalid)
}

func TestSeriesUpdatePrunesStaleOverrides(t *testing.T) {
	env := newTestEnv(t)
	teamID, channelID := mmodel.NewId(), mmodel.NewId()
	creatorID := mmodel.NewId()
	allowChannel(env.api, creatorID, channelID, teamID)
	denyAdmin(env.api, creatorID)

	start := time.Date(2026, time.January, 7, 10, 0, 0, 0, time.UTC).UnixMilli()
	event, err := env.service.Create(nil, "/events", creatorID, taskmodel.CreateEventInput{
		ChannelID:  channelID,
		Visibility: taskmodel.TaskVisibilityChannel,
		Title:      "Standup",
		StartAt:    start,
		EndAt:      start + 3600_000,
		TimeZone:   "UTC",
		Recurrence: &taskmodel.EventRecurrence{Frequency: taskmodel.EventRecurrenceWeekly, Weekdays: []int{3}},
	})
	require.NoError(t, err)

	second := start + 7*24*3600_000
	movedStart := time.Date(2026, time.January, 13, 9, 0, 0, 0, time.UTC).UnixMilli()
	movedEnd := movedStart + 3600_000
	updated, err := env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{
		OccurrenceStartAt: &second,
		StartAt:           &movedStart,
		EndAt:             &movedEnd,
	})
	require.NoError(t, err)
	require.Len(t, updated.Overrides, 1)

	// Re-anchoring the series to Thursdays re-anchors every slot, so the override no longer names
	// a real occurrence and is dropped.
	newStart := start + 24*3600_000
	newEnd := newStart + 3600_000
	updated, err = env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{
		StartAt:    &newStart,
		EndAt:      &newEnd,
		Recurrence: &taskmodel.EventRecurrence{Frequency: taskmodel.EventRecurrenceWeekly, Weekdays: []int{4}},
	})
	require.NoError(t, err)
	assert.Empty(t, updated.Overrides)
}

func TestSeriesUpdateUntilDropsLaterOverrides(t *testing.T) {
	env := newTestEnv(t)
	teamID, channelID := mmodel.NewId(), mmodel.NewId()
	creatorID := mmodel.NewId()
	allowChannel(env.api, creatorID, channelID, teamID)
	denyAdmin(env.api, creatorID)

	// Weekly standup on Wednesdays 10:00 UTC: Jan 7, 14, 21, …
	start := time.Date(2026, time.January, 7, 10, 0, 0, 0, time.UTC).UnixMilli()
	event, err := env.service.Create(nil, "/events", creatorID, taskmodel.CreateEventInput{
		ChannelID:  channelID,
		Visibility: taskmodel.TaskVisibilityChannel,
		Title:      "Standup",
		StartAt:    start,
		EndAt:      start + 3600_000,
		TimeZone:   "UTC",
		Recurrence: &taskmodel.EventRecurrence{Frequency: taskmodel.EventRecurrenceWeekly, Weekdays: []int{3}},
	})
	require.NoError(t, err)

	// The first occurrence is moved, the second cancelled, so both carry overrides.
	second := start + 7*24*3600_000
	firstMoved := time.Date(2026, time.January, 6, 9, 0, 0, 0, time.UTC).UnixMilli()
	firstMovedEnd := firstMoved + 3600_000
	updated, err := env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{
		OccurrenceStartAt: &start,
		StartAt:           &firstMoved,
		EndAt:             &firstMovedEnd,
	})
	require.NoError(t, err)
	cancelled := true
	updated, err = env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{
		OccurrenceStartAt:   &second,
		OccurrenceCancelled: &cancelled,
	})
	require.NoError(t, err)
	require.Len(t, updated.Overrides, 2)

	// Deleting "this and following" from the second occurrence ends the series the day before it:
	// earlier slots and their overrides survive, later overrides no longer name a slot.
	updated, err = env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{
		Recurrence: &taskmodel.EventRecurrence{Frequency: taskmodel.EventRecurrenceWeekly, Weekdays: []int{3}, Until: "2026-01-13"},
	})
	require.NoError(t, err)
	assert.Equal(t, "2026-01-13", updated.Recurrence.Until)
	assert.Equal(t, start, updated.StartAt)
	require.Len(t, updated.Overrides, 1)
	assert.Equal(t, start, updated.Overrides[0].OriginalStart)
	assert.Equal(t, firstMoved, updated.Overrides[0].StartAt)
}

func TestEventLocationIsTrimmedLimitedAndClearable(t *testing.T) {
	env := newTestEnv(t)
	teamID, channelID := mmodel.NewId(), mmodel.NewId()
	creatorID := mmodel.NewId()
	allowChannel(env.api, creatorID, channelID, teamID)
	denyAdmin(env.api, creatorID)

	start := time.Date(2026, time.January, 5, 14, 0, 0, 0, time.UTC).UnixMilli()
	input := taskmodel.CreateEventInput{
		ChannelID: channelID, Visibility: taskmodel.TaskVisibilityChannel, Title: "Sync",
		StartAt: start, EndAt: start + 3600_000, TimeZone: "UTC", Location: "  https://zoom.us/j/123  ",
	}
	event, err := env.service.Create(nil, "/events", creatorID, input)
	require.NoError(t, err)
	assert.Equal(t, "https://zoom.us/j/123", event.Location)

	input.Location = strings.Repeat("x", taskmodel.MaxLocationRunes+1)
	_, err = env.service.Create(nil, "/events", creatorID, input)
	assert.ErrorIs(t, err, taskmodel.ErrInvalid)

	empty := ""
	updated, err := env.service.Update(nil, "/events", creatorID, event.ID, taskmodel.EventPatch{Location: &empty})
	require.NoError(t, err)
	assert.Empty(t, updated.Location)
}
