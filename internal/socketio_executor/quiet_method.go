package socketio_executor

// SetQuiet silences all per-event console output when q is true.
// Call this before Execute to suppress the [RECEIVED] / "Connected" spam
// during load tests so that stdout is not the CPU bottleneck.
func (e *DefaultSocketIOExecutor) SetQuiet(q bool) {
	e.quiet = q
}


func (e *DefaultSocketIOExecutor) SetPauseCh(ch <-chan struct{}) {
	e.pauseMu.Lock()
	e.pauseCh = ch
	e.pauseMu.Unlock()
}


func (e *DefaultSocketIOExecutor) activeCh() <-chan struct{} {
	e.pauseMu.RLock()
	ch := e.pauseCh
	e.pauseMu.RUnlock()
	if ch == nil {
		return alwaysActive
	}
	return ch
}

var alwaysActive = func() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()