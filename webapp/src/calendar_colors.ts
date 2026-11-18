// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {CSSProperties} from 'react';

import type {CalendarColors} from 'types';

// The palette offered next to the color picker. The hexes are chosen to stay readable on both the
// light and dark Mattermost themes.
export const CALENDAR_COLOR_PRESETS = ['#4f7bd9', '#62a461', '#e5a03c', '#e8534a', '#8968cd', '#2fa8a0'];

// The team color separates other people's items from the user's own, whose default is the theme
// accent. An empty stored color always means the default.
export const DEFAULT_CALENDAR_COLOR = {
    mine: '',
    team: CALENDAR_COLOR_PRESETS[1],
};

function hexToRgbTriplet(value: string): string | undefined {
    const match = value.match(/^#([0-9a-f]{6})$/i);
    if (!match) {
        return undefined;
    }
    const raw = parseInt(match[1], 16);
    return `${(raw >> 16) & 255}, ${(raw >> 8) & 255}, ${raw & 255}`;
}

// itemColorVars turns a single item's color into the CSS custom properties its calendar surfaces
// read. An unset color returns nothing so the element falls back to the mine/team defaults.
export function itemColorVars(color?: string): CSSProperties {
    if (!color) {
        return {};
    }
    const vars: Record<string, string> = {'--Tasks-ItemColor': color};
    const rgb = hexToRgbTriplet(color);
    if (rgb) {
        vars['--Tasks-ItemColorRgb'] = rgb;
    }
    return vars as CSSProperties;
}

// calendarColorVars turns the stored colors into CSS custom properties for the calendar root. A
// color that is unset falls through to the defaults declared in calendar.css, so the theme accent
// keeps driving the user's own items until they pick something else.
export function calendarColorVars(colors: CalendarColors | undefined): CSSProperties {
    if (!colors) {
        return {};
    }
    const vars: Record<string, string> = {};
    if (colors.mine) {
        vars['--Tasks-CalendarMine'] = colors.mine;
        const mineRgb = hexToRgbTriplet(colors.mine);
        if (mineRgb) {
            vars['--Tasks-CalendarMineRgb'] = mineRgb;
        }
    }
    if (colors.team) {
        vars['--Tasks-CalendarTeam'] = colors.team;
        const teamRgb = hexToRgbTriplet(colors.team);
        if (teamRgb) {
            vars['--Tasks-CalendarTeamRgb'] = teamRgb;
        }
    }
    return vars as CSSProperties;
}
