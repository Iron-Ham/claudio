package orchestrator

// SetDisplayDimensions sets the initial display dimensions for new instances
// This should be called before the TUI starts to ensure instances are created
// with the correct size from the beginning
func (o *Orchestrator) SetDisplayDimensions(width, height int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.displayWidth = width
	o.displayHeight = height
}

// ResizeAllInstances resizes all running tmux sessions to the given dimensions
// and stores the dimensions for new instances
func (o *Orchestrator) ResizeAllInstances(width, height int) {
	o.mu.Lock()
	o.displayWidth = width
	o.displayHeight = height
	o.mu.Unlock()

	o.mu.RLock()
	defer o.mu.RUnlock()

	for _, mgr := range o.instances {
		if mgr != nil && mgr.Running() {
			_ = mgr.Resize(width, height)
		}
	}
}
