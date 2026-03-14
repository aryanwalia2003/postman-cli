package runner

import "reqx/internal/environment"

// RuntimeContext holds state and variables during execution.
type RuntimeContext struct {
	GlobalVariables map[string]interface{}//this is a map that will hold the global variables
	Environment     *environment.Environment //this is a pointer to the environment struct that will hold the environment variables
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