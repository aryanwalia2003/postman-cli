package runner

import (
	"reqx/internal/environment"
	"sync"
)

// NewRuntimeContext constructs a new RuntimeContext.
func NewRuntimeContext() *RuntimeContext {
	return &RuntimeContext{
		GlobalVariables: make(map[string]interface{}),
		Environment:     environment.NewEnvironment("default"),
		AsyncWG:         new(sync.WaitGroup),
		AsyncStop:       make(chan struct{}),
	}
}
