// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {DAY_IN_MS, localDateKey} from 'calendar_utils';
import type {IntlShape} from 'react-intl';
import {epochToZonedParts, zonedDateTimeToEpoch} from 'timezone_utils';

import type {EventRecurrence, EventSummary} from 'types';

export type CalendarEventOccurrence = {
    event: EventSummary;
    startAt: number;
    endAt: number;
    dateKey: string;

    // For an overridden occurrence, the start the series would have given it.
    originalStart?: number;
};

function utcDateKey(value: number): string {
    const date = new Date(value);
    const pad = (part: number) => String(part).padStart(2, '0');
    return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`;
}

type CivilDate = {
    year: number;
    month: number;
    day: number;
};

function pad(value: number): string {
    return String(value).padStart(2, '0');
}

function civilKey(date: CivilDate): string {
    return `${date.year}-${pad(date.month)}-${pad(date.day)}`;
}

function daysInMonth(year: number, month: number): number {
    return new Date(Date.UTC(year, month, 0)).getUTCDate();
}

function addDays(date: CivilDate, days: number): CivilDate {
    const next = new Date(Date.UTC(date.year, date.month - 1, date.day + days));
    return {year: next.getUTCFullYear(), month: next.getUTCMonth() + 1, day: next.getUTCDate()};
}

function compareDate(a: CivilDate, b: CivilDate): number {
    return civilKey(a).localeCompare(civilKey(b));
}

function weekdayOf(date: CivilDate): number {
    return new Date(Date.UTC(date.year, date.month - 1, date.day)).getUTCDay();
}

function weekStart(date: CivilDate): CivilDate {
    return addDays(date, -weekdayOf(date));
}

function matchesRecurrence(date: CivilDate, anchor: CivilDate, recurrence: EventRecurrence): boolean {
    if (civilKey(date) < civilKey(anchor)) {
        return false;
    }
    if (civilKey(date) === civilKey(anchor)) {
        return true;
    }
    const interval = Math.min(99, Math.max(1, recurrence.interval ?? 1));
    const dayDiff = Math.round((Date.UTC(date.year, date.month - 1, date.day) - Date.UTC(anchor.year, anchor.month - 1, anchor.day)) / DAY_IN_MS);
    switch (recurrence.frequency) {
    case 'daily':
        return dayDiff % interval === 0;
    case 'weekly': {
        const selected = recurrence.weekdays?.length ? recurrence.weekdays : [weekdayOf(anchor)];
        if (!selected.includes(weekdayOf(date))) {
            return false;
        }
        const weekDiff = Math.round((Date.UTC(date.year, date.month - 1, date.day) - Date.UTC(weekStart(anchor).year, weekStart(anchor).month - 1, weekStart(anchor).day)) / (7 * DAY_IN_MS));
        return weekDiff >= 0 && weekDiff % interval === 0;
    }
    case 'monthly': {
        const monthDiff = ((date.year - anchor.year) * 12) + (date.month - anchor.month);
        return monthDiff >= 0 &&
            monthDiff % interval === 0 &&
            date.day === Math.min(anchor.day, daysInMonth(date.year, date.month));
    }
    case 'yearly':
        return date.year >= anchor.year &&
            (date.year - anchor.year) % interval === 0 &&
            date.month === anchor.month &&
            date.day === Math.min(anchor.day, daysInMonth(date.year, date.month));
    default:
        return false;
    }
}

function occurrenceStart(event: EventSummary, date: CivilDate, anchorClock: {hour: number; minute: number}, timeZone: string): number | undefined {
    if (event.allDay) {
        return Date.UTC(date.year, date.month - 1, date.day);
    }
    return zonedDateTimeToEpoch(`${civilKey(date)}T${pad(anchorClock.hour)}:${pad(anchorClock.minute)}`, timeZone);
}

// recurringEvents expands a series into concrete intervals around the visible days. The calendar
// range covers the viewer's local days plus a two-day margin so events in another timezone still
// map to the right local day. Overrides move or cancel single occurrences; an occurrence moved into
// the window from a slot outside it is emitted at its new time.
function recurringEvents(event: EventSummary, days: Date[]): Array<{event: EventSummary; originalStart?: number}> {
    const recurrence = event.recurrence;
    if (!recurrence || recurrence.frequency === 'none' || days.length === 0) {
        return [{event}];
    }
    const timeZone = event.timezone || 'UTC';
    const anchorParts = event.allDay ? (() => {
        const date = new Date(event.startAt);
        return {year: date.getUTCFullYear(), month: date.getUTCMonth() + 1, day: date.getUTCDate(), hour: 0, minute: 0};
    })() : epochToZonedParts(event.startAt, timeZone);
    const anchor: CivilDate = {year: anchorParts.year, month: anchorParts.month, day: anchorParts.day};
    const anchorClock = {hour: anchorParts.hour, minute: anchorParts.minute};
    const firstDay = new Date(Math.min(...days.map((day) => new Date(day.getFullYear(), day.getMonth(), day.getDate()).getTime())));
    const lastDay = new Date(Math.max(...days.map((day) => new Date(day.getFullYear(), day.getMonth(), day.getDate()).getTime())));
    const first: CivilDate = {year: firstDay.getFullYear(), month: firstDay.getMonth() + 1, day: firstDay.getDate()};
    const last: CivilDate = {year: lastDay.getFullYear(), month: lastDay.getMonth() + 1, day: lastDay.getDate()};
    const from = addDays(first, -2);
    const to = addDays(last, 2);
    const duration = event.endAt - event.startAt;
    const until = recurrence.until;
    const overrides = new Map((event.overrides ?? []).map((override) => [override.originalStart, override]));
    const handled = new Set<number>();
    const occurrences: Array<{event: EventSummary; originalStart?: number}> = [];
    for (let date = from; compareDate(date, to) <= 0; date = addDays(date, 1)) {
        if (compareDate(date, anchor) < 0) {
            continue;
        }
        if (until && civilKey(date) > until) {
            break;
        }
        if (!matchesRecurrence(date, anchor, recurrence)) {
            continue;
        }
        const startAt = occurrenceStart(event, date, anchorClock, timeZone);
        if (!startAt) {
            continue;
        }
        const override = overrides.get(startAt);
        if (override) {
            handled.add(startAt);
            if (override.cancelled) {
                continue;
            }
            occurrences.push({event: {...event, startAt: override.startAt, endAt: override.endAt}, originalStart: startAt});
            continue;
        }

        // Every occurrence carries its slot start so the detail view and the editor can act on the
        // occurrence the user clicked, not the series anchor.
        occurrences.push({event: {...event, startAt, endAt: startAt + duration}, originalStart: startAt});
    }

    // Occurrences moved into the visible window from slots the day scan cannot see.
    const civilOf = (value: number): CivilDate | undefined => {
        if (!value) {
            return undefined;
        }
        const parts = event.allDay ? (() => {
            const date = new Date(value);
            return {year: date.getUTCFullYear(), month: date.getUTCMonth() + 1, day: date.getUTCDate()};
        })() : epochToZonedParts(value, timeZone);
        return {year: parts.year, month: parts.month, day: parts.day};
    };
    for (const override of event.overrides ?? []) {
        if (override.cancelled || handled.has(override.originalStart)) {
            continue;
        }
        const date = civilOf(override.startAt);
        if (!date || compareDate(date, from) < 0 || compareDate(date, to) > 0) {
            continue;
        }
        occurrences.push({event: {...event, startAt: override.startAt, endAt: override.endAt}, originalStart: override.originalStart});
    }
    return occurrences;
}

// eventOccurrencesForDays puts an event (or each occurrence of a series) on every visible day it
// overlaps. All-day events use UTC date boundaries and an exclusive end; timed events use the
// viewer's local dates.
export function eventOccurrencesForDays(event: EventSummary, days: Date[]): CalendarEventOccurrence[] {
    if (event.endAt <= event.startAt) {
        return [];
    }
    const occurrences: CalendarEventOccurrence[] = [];
    for (const concrete of recurringEvents(event, days)) {
        const series = concrete.event;
        if (series.endAt <= series.startAt) {
            continue;
        }
        const startKey = series.allDay ? utcDateKey(series.startAt) : localDateKey(new Date(series.startAt));
        const lastMoment = series.endAt - 1;
        const endKey = series.allDay ? utcDateKey(lastMoment) : localDateKey(new Date(lastMoment));
        for (const day of days) {
            const key = localDateKey(day);
            if (key < startKey || key > endKey) {
                continue;
            }
            occurrences.push({
                event: series,
                startAt: series.startAt,
                endAt: series.endAt,
                dateKey: key,
                originalStart: concrete.originalStart,
            });
        }
    }
    return occurrences;
}

export function formatEventTime(intl: IntlShape, event: EventSummary, hour12?: boolean): string {
    if (event.allDay) {
        return intl.formatMessage({id: 'tasks.event.all_day', defaultMessage: 'All day'});
    }
    const start = new Date(event.startAt);
    const end = new Date(event.endAt);
    const startLabel = intl.formatTime(start, {hour: 'numeric', minute: '2-digit', hour12});
    const endLabel = intl.formatTime(end, {hour: 'numeric', minute: '2-digit', hour12});
    if (start.toDateString() === end.toDateString()) {
        return `${startLabel} – ${endLabel}`;
    }
    return `${intl.formatDate(start, {month: 'short', day: 'numeric'})} ${startLabel} – ${intl.formatDate(end, {month: 'short', day: 'numeric'})} ${endLabel}`;
}

// spansDay reports whether the occurrence reaches past the day it is pinned to; those render as a
// band across the calendar, so they lead every day's list to keep their band lane stable.
function spansDay(occurrence: CalendarEventOccurrence): boolean {
    const lastMoment = occurrence.endAt - 1;
    const endKey = occurrence.event.allDay ? utcDateKey(lastMoment) : localDateKey(new Date(lastMoment));
    return endKey > occurrence.dateKey;
}

export function compareEventOccurrences(a: CalendarEventOccurrence, b: CalendarEventOccurrence): number {
    const bandA = spansDay(a) ? 0 : 1;
    const bandB = spansDay(b) ? 0 : 1;
    if (bandA !== bandB) {
        return bandA - bandB;
    }
    if (a.event.allDay !== b.event.allDay) {
        return a.event.allDay ? -1 : 1;
    }
    if (a.startAt !== b.startAt) {
        return a.startAt - b.startAt;
    }
    return a.event.title.localeCompare(b.event.title);
}
