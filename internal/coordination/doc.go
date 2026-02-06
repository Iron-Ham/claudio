// Package coordination provides a Hub that wires all Orchestration 2.0
// components together for a single orchestration session.
//
// The Hub creates and manages the complete task pipeline:
//
//	TaskQueue → EventQueue → Gate
//
// Plus event-driven observers:
//
//   - Adaptive Lead (workload monitoring and rebalancing)
//   - Scaling Monitor (elastic instance scaling decisions)
//
// And communication infrastructure:
//
//   - Context Propagator (cross-instance knowledge sharing)
//   - File Lock Registry (conflict prevention)
//   - Mailbox (underlying message transport)
//
// Usage:
//
//	hub, err := coordination.NewHub(coordination.Config{
//	    Bus:        bus,
//	    SessionDir: sessionDir,
//	    Plan:       planSpec,
//	    TaskLookup: lookupFunc,
//	})
//	if err != nil {
//	    return err
//	}
//	if err := hub.Start(ctx); err != nil {
//	    return err
//	}
//	defer hub.Stop()
//
//	// Use hub.Gate() for task operations
//	task, err := hub.Gate().ClaimNext("instance-1")
package coordination
