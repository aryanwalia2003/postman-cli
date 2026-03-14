package storage

import (
	"encoding/json"
	"reqx/internal/collection"
)

// ParseCollection takes raw JSON bytes and returns a parsed Collection.
func ParseCollection(data []byte) (*collection.Collection, error) {
	var coll collection.Collection //coll is a collection.Collection type
	err := json.Unmarshal(data, &coll)
	if err != nil {
		return nil, err
	}
	return &coll, nil
}
