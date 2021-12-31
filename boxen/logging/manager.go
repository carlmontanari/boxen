package logging

// Manager is a singleton/global log manager instance.
var Manager = &manager{} //nolint:gochecknoglobals

// manager is the instance that holds all loggers.
type manager struct {
	loggers []*Instance
}

// AddInstance adds a logging instance *Instance to the log manager.
func (l *manager) addInstance(li *Instance) {
	li.Start()

	l.loggers = append(l.loggers, li)
}

// Terminate terminates all the logging instances that the manager holds.
func (l *manager) Terminate() {
	for _, li := range l.loggers {
		// iterate over all loggers and set the done flag, so we can terminate the program and all
		// logger goroutines
		li.setDone(true)
	}

	for _, li := range l.loggers {
		li.wg.Wait()
	}
}
