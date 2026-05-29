package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/lib/pq"
)

// JSONMap is a flexible key-value store backed by JSONB in Postgres.
type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("JSONMap: unsupported type %T", value)
	}
	return json.Unmarshal(bytes, j)
}

// StringArray is a native PostgreSQL text[] array.
// It delegates to pq.StringArray for proper array serialisation/deserialisation,
// which avoids the fragile comma-join approach and supports values that contain commas.
type StringArray = pq.StringArray
