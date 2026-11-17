package api

import (
	"encoding/json"
	"net/http"

	taskmodel "github.com/3tty0n/mattermost-tasks/server/model"
)

// handleGetCalendarColors serves the signed-in user's calendar colors. A user without preferences
// gets the zero value; the webapp fills in the defaults.
func (h *Handler) handleGetCalendarColors(w http.ResponseWriter, r *http.Request) {
	colors, err := h.prefs.CalendarColors(r.Header.Get("Mattermost-User-ID"))
	if err != nil {
		writeServiceError(w, r, err, "Failed to load calendar colors")
		return
	}
	writeJSON(w, r, http.StatusOK, colors)
}

// handleSetCalendarColors stores the signed-in user's calendar colors. The colors are validated
// server-side because they end up in a CSS property.
func (h *Handler) handleSetCalendarColors(w http.ResponseWriter, r *http.Request) {
	var input taskmodel.CalendarColors
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)).Decode(&input); err != nil {
		writeServiceError(w, r, taskmodel.ErrInvalid, "Invalid request body")
		return
	}
	colors, err := h.prefs.SetCalendarColors(r.Header.Get("Mattermost-User-ID"), input)
	if err != nil {
		writeServiceError(w, r, err, "Failed to save calendar colors")
		return
	}
	writeJSON(w, r, http.StatusOK, colors)
}
