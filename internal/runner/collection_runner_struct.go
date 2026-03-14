package runner

import (
	"postman-cli/internal/http_executor"
	"postman-cli/internal/scripting"
	"postman-cli/internal/socketio_executor"
	"time"
)

type RequestMetric struct{
	Name string
	Protocol string
	StatusCode int
	Duration time.Duration
	StatusString string
	Error error
}
// CollectionRunner handles executing a full collection of requests.
type CollectionRunner struct {
	executor              *http_executor.DefaultExecutor
	sioExecutor           socketio_executor.SocketIOExecutor
	scriptRunner          scripting.ScriptRunner
	clearCookiesPerRequest bool // if true, jar is cleared before each request
	verboseMode           bool
	metrics 			  []RequestMetric 
}


