package runner

import (
	"reqx/internal/environment"
	"sync"
)

// RuntimeContext holds state and variables during execution.
type RuntimeContext struct {
	GlobalVariables map[string]interface{}//this is a map that will hold the global variables
	Environment     *environment.Environment //this is a pointer to the environment struct that will hold the environment variables
	AsyncWG         *sync.WaitGroup          // Shared across DAG parallel nodes to track background tasks
	AsyncStop       chan struct{}            // Shared across DAG parallel nodes to signal background stop
	AsyncStopOnce   sync.Once                // Ensures AsyncStop is closed exactly once
}

//this struct looks like 
// {
// 	"globalVariables": {
// 		"key": "value"
// 	},
// 	"environment": {
// 		"name": "dev",
// 		"variables": {
// 			"key": "value"
// 		}
// 	}
// }

//this struct is used 