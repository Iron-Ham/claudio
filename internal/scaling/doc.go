// Package scaling provides queue-depth-based elastic scaling decisions for
// Ultra-Plan execution.
//
// During plan execution, the number of active instances may need to grow or
// shrink based on workload. The scaling package monitors queue depth events
// and applies a configurable policy to recommend scaling actions.
//
// The core types are:
//
//   - [Policy]: Defines scaling rules (thresholds, cooldown, instance limits)
//   - [Monitor]: Watches queue depth events on the event bus and applies the policy
//   - [Decision]: The output of policy evaluation — scale up, scale down, or hold
//
// # Usage
//
//	policy := scaling.NewPolicy(
//	    scaling.WithMinInstances(1),
//	    scaling.WithMaxInstances(8),
//	    scaling.WithScaleUpThreshold(2),
//	    scaling.WithScaleDownThreshold(1),
//	    scaling.WithCooldownPeriod(30 * time.Second),
//	)
//
//	monitor := scaling.NewMonitor(bus, policy)
//	monitor.OnDecision(func(d scaling.Decision) {
//	    log.Printf("Scaling: %s delta=%d reason=%s", d.Action, d.Delta, d.Reason)
//	})
//	monitor.Start(ctx)
//	defer monitor.Stop()
//
// # Thread Safety
//
// All types in this package are safe for concurrent use.
package scaling
