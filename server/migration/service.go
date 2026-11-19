// Package migration copies raw KV records between two plugin installs. The Mattermost plugin KV
// store namespaces records by plugin ID, so renaming the plugin ID leaves the old records
// unreachable: the old install exports them and the new one imports them.
package migration

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost/server/public/plugin"

	taskmodel "github.com/3tty0n/mattermost-tasks/server/model"
	"github.com/3tty0n/mattermost-tasks/server/store/kvstore"
)

// exportFormat is the JSON shape of an export file. Values are base64 because records are
// opaque bytes: tasks, indexes, reminders and preferences all mix JSON shapes.
type exportFormat struct {
	Version int          `json:"version"`
	Keys    []exportPair `json:"keys"`
}

type exportPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Service implements the export/import use cases.
type Service struct {
	store kvstore.MigrationStore
}

// NewService builds a Service on top of the plugin KV store.
func NewService(store kvstore.MigrationStore) *Service {
	return &Service{store: store}
}

// Export reads every record in the namespace and returns the export file's bytes.
func (s *Service) Export(c *plugin.Context, actorID, path string) ([]byte, error) {
	keys, err := s.store.ListAllKeys()
	if err != nil {
		return nil, err
	}
	out := exportFormat{Version: 1}
	for _, key := range keys {
		raw, err := s.store.ReadRaw(key)
		if err != nil {
			if err == kvstore.ErrNotFound {
				continue
			}
			return nil, err
		}
		out.Keys = append(out.Keys, exportPair{Key: key, Value: base64.StdEncoding.EncodeToString(raw)})
	}
	return json.MarshalIndent(out, "", "  ")
}

// Import writes the export file's records. A key that already exists in the target namespace is
// skipped, so re-running an import never overwrites live data. It returns how many records were
// written and skipped.
func (s *Service) Import(c *plugin.Context, actorID, path string, body []byte) (written, skipped int, err error) {
	var input exportFormat
	if err := json.Unmarshal(body, &input); err != nil {
		return 0, 0, fmt.Errorf("%w: invalid import file", taskmodel.ErrInvalid)
	}
	if input.Version != 1 {
		return 0, 0, fmt.Errorf("%w: unknown export version", taskmodel.ErrInvalid)
	}
	for _, pair := range input.Keys {
		if pair.Key == "" {
			return written, skipped, fmt.Errorf("%w: empty key in import", taskmodel.ErrInvalid)
		}
		exists, err := s.store.HasKey(pair.Key)
		if err != nil {
			return written, skipped, err
		}
		if exists {
			skipped++
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(pair.Value)
		if err != nil {
			return written, skipped, fmt.Errorf("%w: invalid value for %s", taskmodel.ErrInvalid, pair.Key)
		}
		if err := s.store.WriteRaw(pair.Key, raw); err != nil {
			return written, skipped, err
		}
		written++
	}
	return written, skipped, nil
}
