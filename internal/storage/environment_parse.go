package storage

import (
	"encoding/json"
	"reqx/internal/environment"
)

// ParseEnvironment takes raw JSON bytes and returns a parsed Environment.
func ParseEnvironment(data []byte) (*environment.Environment, error) {
	var env environment.Environment //zero object banaya of type enviroment 
	err := json.Unmarshal(data, &env)
	if err != nil {
		return nil, err
	}
	return &env, nil
}
