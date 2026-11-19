package kvstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationRoundTrip(t *testing.T) {
	s := NewStoreWithKV(NewMemoryKV())
	require.NoError(t, s.WriteRaw("task:abc", []byte(`{"id":"abc"}`)))
	require.NoError(t, s.WriteRaw("event:def", []byte(`{"id":"def"}`)))

	keys, err := s.ListAllKeys()
	require.NoError(t, err)
	assert.Equal(t, []string{"event:def", "task:abc"}, keys)

	raw, err := s.ReadRaw("task:abc")
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"abc"}`, string(raw))

	_, err = s.ReadRaw("task:missing")
	assert.ErrorIs(t, err, ErrNotFound)

	has, err := s.HasKey("event:def")
	require.NoError(t, err)
	assert.True(t, has)

	// WriteRaw replaces the bytes as-is, so an import into an empty store restores the record.
	other := NewStoreWithKV(NewMemoryKV())
	require.NoError(t, other.WriteRaw("task:abc", raw))
	restored, err := other.ReadRaw("task:abc")
	require.NoError(t, err)
	assert.Equal(t, raw, restored)
}
