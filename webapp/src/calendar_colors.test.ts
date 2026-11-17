// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {calendarColorVars} from 'calendar_colors';

describe('calendarColorVars', () => {
    it('leaves the defaults in place when nothing is set', () => {
        expect(calendarColorVars(undefined)).toEqual({});
        expect(calendarColorVars({})).toEqual({});
    });

    it('sets each color with its rgb triplet for the tinted surfaces', () => {
        expect(calendarColorVars({mine: '#4f7bd9', team: '#62a461'})).toEqual({
            '--Tasks-CalendarMine': '#4f7bd9',
            '--Tasks-CalendarMineRgb': '79, 123, 217',
            '--Tasks-CalendarTeam': '#62a461',
            '--Tasks-CalendarTeamRgb': '98, 164, 97',
        });
    });

    it('ignores colors that are not hex values', () => {
        const vars = calendarColorVars({team: 'nope'}) as Record<string, string>;
        expect(vars['--Tasks-CalendarTeam']).toBe('nope');
        expect(vars['--Tasks-CalendarTeamRgb']).toBeUndefined();
    });
});
