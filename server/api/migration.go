package api

import (
	"io"
	"net/http"
)

// maxImportBodyBytes bounds the import upload. An export of a few thousand tasks and events with
// descriptions is a few megabytes; 32 MiB leaves wide margin without accepting unbounded bodies.
const maxImportBodyBytes = 32 << 20

// handleExportMigrations serves every KV record of this plugin's namespace as a JSON file. The
// payload carries all users' data, so only a system administrator may call it.
func (h *Handler) handleExportMigration(w http.ResponseWriter, r *http.Request) {
	actorID := r.Header.Get("Mattermost-User-ID")
	if !h.access.IsAdmin(actorID) {
		writeError(w, r, http.StatusForbidden, "You don't have permission to do that", nil)
		return
	}
	data, err := h.migration.Export(pluginContext(r), actorID, r.URL.Path)
	if err != nil {
		writeServiceError(w, r, err, "Failed to export the plugin data")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="agenda-export.json"`)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		requestLogger(r).WithError(err).Warnf("Failed to write export")
	}
}

// handleImportMigration writes an export file's records into this namespace. Existing keys are
// skipped, so an import never overwrites live data. System administrators only.
func (h *Handler) handleImportMigration(w http.ResponseWriter, r *http.Request) {
	actorID := r.Header.Get("Mattermost-User-ID")
	if !h.access.IsAdmin(actorID) {
		writeError(w, r, http.StatusForbidden, "You don't have permission to do that", nil)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxImportBodyBytes))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "Invalid request body", nil)
		return
	}
	written, skipped, err := h.migration.Import(pluginContext(r), actorID, r.URL.Path, body)
	if err != nil {
		writeServiceError(w, r, err, "Failed to import the plugin data")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]int{"written": written, "skipped": skipped})
}
