package migration

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	taskmodel "github.com/3tty0n/mattermost-tasks/server/model"
	"github.com/3tty0n/mattermost-tasks/server/store/kvstore"
)

func TestExportImportRoundTrip(t *testing.T) {
	source := kvstore.NewStoreWithKV(kvstore.NewMemoryKV())
	require.NoError(t, source.WriteRaw("task:abc", []byte(`{"id":"abc"}`)))
	require.NoError(t, source.WriteRaw("event:def", []byte(`{"id":"def"}`)))
	require.NoError(t, source.WriteRaw("reminder:100", []byte(`[{"k":"task"}]`)))

	exporter := NewService(source)
	data, err := exporter.Export(nil, "user-1", "/export")
	require.NoError(t, err)

	// The file is the documented shape: version plus base64 pairs.
	var parsed struct {
		Version int `json:"version"`
		Keys    []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, 1, parsed.Version)
	require.Len(t, parsed.Keys, 3)

	target := kvstore.NewStoreWithKV(kvstore.NewMemoryKV())
	restorer := NewService(target)
	written, skipped, err := restorer.Import(nil, "user-1", "/import", data)
	require.NoError(t, err)
	assert.Equal(t, 3, written)
	assert.Zero(t, skipped)

	for _, key := range []string{"task:abc", "event:def", "reminder:100"} {
		raw, err := target.ReadRaw(key)
		require.NoError(t, err, key)
		_, err = base64.StdEncoding.DecodeString(string(raw)) //nolint:staticcheck
		require.Error(t, err, "records are stored decoded, not base64")
	}

	// Importing again skips every key instead of overwriting live records.
	written, skipped, err = restorer.Import(nil, "user-1", "/import", data)
	require.NoError(t, err)
	assert.Zero(t, written)
	assert.Equal(t, 3, skipped)
}

func TestImportValidation(t *testing.T) {
	store := kvstore.NewStoreWithKV(kvstore.NewMemoryKV())
	service := NewService(store)

	_, _, err := service.Import(nil, "user-1", "/import", []byte(`{"version":2,"keys":[]}`))
	assert.ErrorIs(t, err, taskmodel.ErrInvalid)

	_, _, err = service.Import(nil, "user-1", "/import", []byte(`not json`))
	assert.ErrorIs(t, err, taskmodel.ErrInvalid)

	// Empty keys and invalid base64 are rejected rather than half-applied.
	_, _, err = service.Import(nil, "user-1", "/import", []byte(`{"version":1,"keys":[{"key":"","value":""}]}`))
	assert.ErrorIs(t, err, taskmodel.ErrInvalid)
	_, _, err = service.Import(nil, "user-1", "/import", []byte(`{"version":1,"keys":[{"key":"task:x","value":"!!!"}]}`))
	assert.ErrorIs(t, err, taskmodel.ErrInvalid)
	keys, err := store.ListAllKeys()
	require.NoError(t, err)
	assert.Empty(t, keys)
}
