// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {CALENDAR_COLOR_PRESETS, DEFAULT_CALENDAR_COLOR} from 'calendar_colors';
import {useEffect, useLayoutEffect, useRef, useState} from 'react';
import {createPortal} from 'react-dom';
import {FormattedMessage, useIntl} from 'react-intl';

import PaletteOutlineIcon from '@mattermost/compass-icons/components/palette-outline';
import {Icon} from '@mattermost/compass-ui/components/icon';
import {IconButton} from '@mattermost/compass-ui/components/icon-button';

import type {CalendarColors} from 'types';

import './calendar_color_settings.css';

type Row = 'mine' | 'team';

type Props = {
    colors: CalendarColors;
    disabled?: boolean;
    onChange: (next: CalendarColors) => void;
};

// CalendarColorSettings opens the popover that colors the user's own calendar items separately
// from everyone else's. Presets cover the common choice; the native color picker covers the rest.
export default function CalendarColorSettings({colors, disabled, onChange}: Props) {
    const intl = useIntl();
    const wrapperRef = useRef<HTMLDivElement>(null);
    const popoverRef = useRef<HTMLDivElement>(null);
    const [open, setOpen] = useState(false);

    useEffect(() => {
        if (!open) {
            return undefined;
        }
        const close = (event: MouseEvent) => {
            const target = event.target as Node;
            if (!wrapperRef.current?.contains(target) && !popoverRef.current?.contains(target)) {
                setOpen(false);
            }
        };
        document.addEventListener('mousedown', close);
        return () => document.removeEventListener('mousedown', close);
    }, [open]);

    useLayoutEffect(() => {
        if (!open) {
            return undefined;
        }
        const update = () => {
            const trigger = wrapperRef.current?.getBoundingClientRect();
            if (!trigger || !popoverRef.current) {
                return;
            }
            let top = trigger.bottom + 4;
            if (top + popoverRef.current.offsetHeight > window.innerHeight - 8) {
                top = Math.max(8, trigger.top - popoverRef.current.offsetHeight - 4);
            }
            const left = Math.max(8, Math.min(trigger.left, window.innerWidth - popoverRef.current.offsetWidth - 8));
            popoverRef.current.style.top = `${top}px`;
            popoverRef.current.style.left = `${left}px`;
        };
        update();
        window.addEventListener('resize', update);
        window.addEventListener('scroll', update, true);
        return () => {
            window.removeEventListener('resize', update);
            window.removeEventListener('scroll', update, true);
        };
    }, [open]);

    const rowLabel = (row: Row) => (
        <FormattedMessage
            id={row === 'mine' ? 'tasks.calendar.colors.mine' : 'tasks.calendar.colors.team'}
            defaultMessage={row === 'mine' ? 'My items' : 'Team items'}
        />
    );

    const current = (row: Row) => colors[row] ?? '';

    const select = (row: Row, value: string) => {
        onChange({...colors, [row]: value});
    };

    const renderRow = (row: Row) => {
        const selected = current(row);
        return (
            <div
                key={row}
                className='Tasks-CalendarColorSettings__row'
            >
                <span className='Tasks-CalendarColorSettings__rowLabel'>{rowLabel(row)}</span>
                <div className='Tasks-CalendarColorSettings__swatches'>
                    {CALENDAR_COLOR_PRESETS.map((preset) => (
                        <button
                            key={preset}
                            type='button'
                            className={`Tasks-CalendarColorSettings__swatch${selected === preset ? ' Tasks-CalendarColorSettings__swatch--selected' : ''}`}
                            style={{backgroundColor: preset}}
                            aria-pressed={selected === preset}
                            aria-label={intl.formatMessage(
                                {id: 'tasks.calendar.colors.select', defaultMessage: 'Set the {which} color to {value}'},
                                {which: intl.formatMessage(row === 'mine' ? {id: 'tasks.calendar.colors.mine', defaultMessage: 'My items'} : {id: 'tasks.calendar.colors.team', defaultMessage: 'Team items'}),
                                    value: preset},
                            )}
                            onClick={() => select(row, preset)}
                        />
                    ))}
                    <label
                        className={`Tasks-CalendarColorSettings__custom${selected && !CALENDAR_COLOR_PRESETS.includes(selected) ? ' Tasks-CalendarColorSettings__custom--selected' : ''}`}
                    >
                        <input
                            type='color'
                            value={(/^#[0-9a-f]{6}$/i).test(selected) ? selected : DEFAULT_CALENDAR_COLOR.team}
                            aria-label={intl.formatMessage(
                                {id: 'tasks.calendar.colors.custom', defaultMessage: 'Pick a custom color for {which}'},
                                {which: intl.formatMessage(row === 'mine' ? {id: 'tasks.calendar.colors.mine', defaultMessage: 'My items'} : {id: 'tasks.calendar.colors.team', defaultMessage: 'Team items'})},
                            )}
                            onChange={(changeEvent) => select(row, changeEvent.target.value.toLowerCase())}
                        />
                    </label>
                    <button
                        type='button'
                        className='Tasks-CalendarColorSettings__default'
                        aria-label={intl.formatMessage(
                            {id: 'tasks.calendar.colors.reset', defaultMessage: 'Reset the {which} color'},
                            {which: intl.formatMessage(row === 'mine' ? {id: 'tasks.calendar.colors.mine', defaultMessage: 'My items'} : {id: 'tasks.calendar.colors.team', defaultMessage: 'Team items'})},
                        )}
                        onClick={() => select(row, '')}
                    >
                        <FormattedMessage
                            id='tasks.calendar.colors.default'
                            defaultMessage='Default'
                        />
                    </button>
                </div>
            </div>
        );
    };

    return (
        <div
            ref={wrapperRef}
            className='Tasks-CalendarColorSettings'
        >
            <IconButton
                size='small'
                icon={<Icon glyph={<PaletteOutlineIcon/>}/>}
                aria-label={intl.formatMessage({id: 'tasks.calendar.colors.title', defaultMessage: 'Calendar colors'})}
                aria-expanded={open}
                aria-haspopup='dialog'
                disabled={disabled}
                onClick={() => setOpen((value) => !value)}
            />
            {open && createPortal(
                <div
                    ref={popoverRef}
                    className='Tasks-CalendarColorSettings__popover'
                    role='dialog'
                    aria-label={intl.formatMessage({id: 'tasks.calendar.colors.title', defaultMessage: 'Calendar colors'})}
                >
                    <h4 className='Tasks-CalendarColorSettings__heading'>
                        <FormattedMessage
                            id='tasks.calendar.colors.title'
                            defaultMessage='Calendar colors'
                        />
                    </h4>
                    {renderRow('mine')}
                    {renderRow('team')}
                    <p className='Tasks-CalendarColorSettings__hint'>
                        <FormattedMessage
                            id='tasks.calendar.colors.hint'
                            defaultMessage='Your items get one color, everyone else’s another. The colors follow you across devices.'
                        />
                    </p>
                </div>,
                document.body,
            )}
        </div>
    );
}
