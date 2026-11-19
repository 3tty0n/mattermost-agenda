package kvstore

// listPageSize is how many keys one KVList page reads while exporting.
const listPageSize = 200

// MigrationStore moves raw KV records between plugin installs. It exists for the plugin-ID
// rename: a plugin can only read its own namespace, so the old install exports every record and
// the new one imports them.
type MigrationStore interface {
	// ListAllKeys pages through the whole namespace.
	ListAllKeys() ([]string, error)
	// ReadRaw returns a record exactly as stored, or ErrNotFound.
	ReadRaw(key string) ([]byte, error)
	// HasKey reports whether a record exists, without decoding it.
	HasKey(key string) (bool, error)
	// WriteRaw stores bytes under key.
	WriteRaw(key string, value []byte) error
}

var _ MigrationStore = (*KVStore)(nil)

// ListAllKeys pages through the namespace until the store stops returning keys.
func (s *KVStore) ListAllKeys() ([]string, error) {
	var keys []string
	for page := 0; ; page++ {
		batch, err := s.kv.List(page, listPageSize)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return keys, nil
		}
		keys = append(keys, batch...)
	}
}

// ReadRaw returns the record bytes. A nil result means the key does not exist.
func (s *KVStore) ReadRaw(key string) ([]byte, error) {
	var raw []byte
	if err := s.kv.Get(key, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, ErrNotFound
	}
	return raw, nil
}

// HasKey reports whether a record exists.
func (s *KVStore) HasKey(key string) (bool, error) {
	_, err := s.ReadRaw(key)
	if err == nil {
		return true, nil
	}
	if err == ErrNotFound {
		return false, nil
	}
	return false, err
}

// WriteRaw stores the record bytes as-is.
func (s *KVStore) WriteRaw(key string, value []byte) error {
	_, err := s.kv.Set(key, append([]byte(nil), value...))
	return err
}
