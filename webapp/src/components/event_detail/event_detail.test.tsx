// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {getEvent, getProfiles} from 'client';
import {downloadIcs} from 'event_export';

import type {Event} from 'types';

import EventDetail from './event_detail';

import {renderWithStore, ids} from '../../../tests/test_utils';

jest.mock('client', () => ({
    getEvent: jest.fn(),
    getProfiles: jest.fn(),
}));
jest.mock('event_export', () => ({
    ...jest.requireActual('event_export'),
    downloadIcs: jest.fn(),
}));

const event = (creatorId: string): Event => ({
    id: 'e1',
    teamId: ids.team,
    visibility: 'team',
    creatorId,
    assigneeIds: [],
    title: 'Sprint planning',
    description: 'Agenda **first**',
    location: 'https://zoom.us/j/1',
    startAt: Date.UTC(2026, 0, 5, 9),
    endAt: Date.UTC(2026, 0, 5, 10),
    timezone: 'UTC',
    createdAt: 1,
    updatedAt: 1,
    version: 1,
});

describe('EventDetail', () => {
    beforeEach(() => {
        (getProfiles as jest.Mock).mockResolvedValue({});
    });

    it('opens read-only and lets the creator switch to editing', async () => {
        (getEvent as jest.Mock).mockResolvedValue(event(ids.user));
        const onEdit = jest.fn();
        renderWithStore(
            <EventDetail
                eventId='e1'
                onClose={jest.fn()}
                onEdit={onEdit}
            />,
        );
        expect(await screen.findByText('Sprint planning')).toBeVisible();
        expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
        expect(screen.getByRole('link', {name: 'https://zoom.us/j/1'})).toBeVisible();
        await userEvent.click(screen.getByRole('button', {name: 'Edit'}));
        expect(onEdit).toHaveBeenCalled();
    });

    it('hides Edit from others and offers the calendar exports', async () => {
        (getEvent as jest.Mock).mockResolvedValue(event(ids.otherUser));
        const open = jest.spyOn(window, 'open').mockReturnValue(null);
        renderWithStore(
            <EventDetail
                eventId='e1'
                onClose={jest.fn()}
                onEdit={jest.fn()}
            />,
        );
        await screen.findByText('Sprint planning');
        expect(screen.queryByRole('button', {name: 'Edit'})).not.toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'Google Calendar'}));
        expect(open).toHaveBeenCalledWith(expect.stringContaining('calendar.google.com'), '_blank', 'noopener,noreferrer');
        await userEvent.click(screen.getByRole('button', {name: 'Outlook'}));
        expect(open).toHaveBeenCalledWith(expect.stringContaining('outlook.office.com'), '_blank', 'noopener,noreferrer');
        await userEvent.click(screen.getByRole('button', {name: 'Download .ics'}));
        await waitFor(() => expect(downloadIcs).toHaveBeenCalled());
        open.mockRestore();
    });

    describe('with an occurrence of a recurring series', () => {
        const recurring: Event = {
            ...event(ids.user),
            recurrence: {frequency: 'weekly', interval: 1, weekdays: [1]},
        };
        const slot = Date.UTC(2026, 0, 12, 9); // the next Monday occurrence
        const movedTo = Date.UTC(2026, 0, 13, 15); // moved to Tuesday 15:00

        it('shows the occurrence’s own times, including a moved override', async () => {
            (getEvent as jest.Mock).mockResolvedValue({
                ...recurring,
                overrides: [{originalStart: slot, startAt: movedTo, endAt: movedTo + 3600000}],
            });
            renderWithStore(
                <EventDetail
                    eventId='e1'
                    occurrenceStartAt={slot}
                    onClose={jest.fn()}
                    onEdit={jest.fn()}
                />,
            );
            expect(await screen.findByText(/Jan 13, 2026/)).toBeVisible();
        });

        it('exports the single occurrence without the series rule', async () => {
            (getEvent as jest.Mock).mockResolvedValue(recurring);
            const open = jest.spyOn(window, 'open').mockReturnValue(null);
            renderWithStore(
                <EventDetail
                    eventId='e1'
                    occurrenceStartAt={slot}
                    onClose={jest.fn()}
                    onEdit={jest.fn()}
                />,
            );
            await screen.findByText('Sprint planning');
            await userEvent.click(screen.getByRole('button', {name: 'Google Calendar'}));
            expect(open).toHaveBeenCalledWith(expect.not.stringContaining('recur'), '_blank', 'noopener,noreferrer');
            open.mockRestore();
        });
    });
});
