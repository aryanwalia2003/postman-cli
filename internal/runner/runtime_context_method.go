package runner

import "reqx/internal/environment"

// SetGlobalVariable sets a variable in the global context.
func (rc *RuntimeContext) SetGlobalVariable(key string, value interface{}) {
	if rc.GlobalVariables == nil {
		rc.GlobalVariables = make(map[string]interface{})
	}
	rc.GlobalVariables[key] = value
}

// GetVariable attempts to find a variable, checking globals first, then environment.
func (rc *RuntimeContext) GetVariable(key string) (interface{}, bool) {
	// Check global variables
	if rc.GlobalVariables != nil {
		if val, exists := rc.GlobalVariables[key]; exists {
			return val, true
		}
	}

	// Check environment variables
	if rc.Environment != nil {
		if val, exists := rc.Environment.Get(key); exists {
			return val, true
		}
	}

	return nil, false
}

// SetEnvironment configures the active environment for this context.
func (rc *RuntimeContext) SetEnvironment(env *environment.Environment) {
	rc.Environment = env
}
