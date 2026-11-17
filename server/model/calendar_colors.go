package model

import (
	"fmt"
	"strings"
)

// CalendarColors are a user's own calendar colors: Mine tints items that user created, Team the
// ones other people created. Values are lowercase #rrggbb; empty means the client default.
type CalendarColors struct {
	Mine string `json:"mine"`
	Team string `json:"team"`
}

// NormalizeCalendarColors validates both colors and returns their lowercase form. An empty color
// is valid and means "use the default".
func NormalizeCalendarColors(input CalendarColors) (CalendarColors, error) {
	mine, err := normalizeHexColor(input.Mine)
	if err != nil {
		return CalendarColors{}, err
	}
	team, err := normalizeHexColor(input.Team)
	if err != nil {
		return CalendarColors{}, err
	}
	return CalendarColors{Mine: mine, Team: team}, nil
}

// normalizeHexColor accepts an optional #rrggbb value, case-insensitively, and returns it in
// lowercase. Anything else is invalid: the value ends up in a CSS property, so it must not carry
// another syntax the client would have to sanitize.
func normalizeHexColor(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	if len(value) != 7 || value[0] != '#' {
		return "", fmt.Errorf("%w: invalid color", ErrInvalid)
	}
	for _, c := range value[1:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", fmt.Errorf("%w: invalid color", ErrInvalid)
		}
	}
	return value, nil
}
