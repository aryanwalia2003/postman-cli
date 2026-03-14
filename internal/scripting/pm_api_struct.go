package scripting

import (
	"reqx/internal/environment"
)

// Yeh basically postman ka pm object hai , yeh apne andar enviroment rakhta hai
type PmAPI struct {
	Environment *EnvironmentAPI `json:"environment"`
	Response    *ResponseAPI    `json:"response"`
	// For tracking pm.test calls
	TestResults *TestResults    `json:"testResults"`
}

// EnvironmentAPI encapsulates `pm.environment.*`
type EnvironmentAPI struct {
	env *environment.Environment
}

func (e *EnvironmentAPI) Get(key string) string {
	if e.env == nil {
		return ""
	}
	v, ok := e.env.Get(key)
	if !ok {
		return ""
	}
	return v
}

func (e *EnvironmentAPI) Set(key, value string) {
	if e.env != nil {
		e.env.Set(key, value)
	}
}

func (e *EnvironmentAPI) Unset(key string) {
	if e.env != nil {
		e.env.Unset(key)
	}
}
//this will be used for pm.test("name", function() { ... })
func (p *PmAPI) Test(name string, fn func()) {
	// Let's execute the function immediately. If it panics/fails, it will be caught.
	// A more advanced engine would use a try/catch mechanism, but Goja can handle standard traps.
	
	// Because Goja translates JS exceptions to Go errors, we will just track
	// passes and failures via our Expect library side-effects for now.

	//!TODO
	
	// This initial implementation registers the test and assumes pass unless expect() flags otherwise.
	if p.TestResults != nil {
		*p.TestResults = append(*p.TestResults, TestResult{
			Name:   name,
			Passed: true, // Optimistically assuming pass
			Error:  "",
		})
	}

	// Trigger the user's JS callback
	fn()
}

// Expect provides a lightweight assertion builder directly integrated with Goja.
func (p *PmAPI) Expect(value interface{}) ExpectBuilder {
	return &defaultExpectBuilder{
		value:       value,
		testResults: p.TestResults,
	}
}
