package scaling

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/taskqueue"
)

func TestNewPolicy_Defaults(t *testing.T) {
	p := NewPolicy()
	if p.minInstances != defaultMinInstances {
		t.Errorf("minInstances = %d, want %d", p.minInstances, defaultMinInstances)
	}
	if p.maxInstances != defaultMaxInstances {
		t.Errorf("maxInstances = %d, want %d", p.maxInstances, defaultMaxInstances)
	}
	if p.scaleUpThreshold != defaultScaleUpThreshold {
		t.Errorf("scaleUpThreshold = %d, want %d", p.scaleUpThreshold, defaultScaleUpThreshold)
	}
	if p.scaleDownThreshold != defaultScaleDownThreshold {
		t.Errorf("scaleDownThreshold = %d, want %d", p.scaleDownThreshold, defaultScaleDownThreshold)
	}
	if p.cooldownPeriod != defaultCooldownPeriod {
		t.Errorf("cooldownPeriod = %v, want %v", p.cooldownPeriod, defaultCooldownPeriod)
	}
}

func TestNewPolicy_Options(t *testing.T) {
	p := NewPolicy(
		WithMinInstances(2),
		WithMaxInstances(16),
		WithScaleUpThreshold(5),
		WithScaleDownThreshold(3),
		WithCooldownPeriod(time.Minute),
	)
	if p.minInstances != 2 {
		t.Errorf("minInstances = %d, want 2", p.minInstances)
	}
	if p.maxInstances != 16 {
		t.Errorf("maxInstances = %d, want 16", p.maxInstances)
	}
	if p.scaleUpThreshold != 5 {
		t.Errorf("scaleUpThreshold = %d, want 5", p.scaleUpThreshold)
	}
	if p.scaleDownThreshold != 3 {
		t.Errorf("scaleDownThreshold = %d, want 3", p.scaleDownThreshold)
	}
	if p.cooldownPeriod != time.Minute {
		t.Errorf("cooldownPeriod = %v, want %v", p.cooldownPeriod, time.Minute)
	}
}

func TestPolicy_Evaluate(t *testing.T) {
	tests := []struct {
		name             string
		status           taskqueue.QueueStatus
		currentInstances int
		options          []Option
		wantAction       Action
		wantDeltaSign    int // -1, 0, +1
	}{
		{
			name: "scale up when pending exceeds running",
			status: taskqueue.QueueStatus{
				Pending: 5,
				Running: 2,
				Total:   10,
			},
			currentInstances: 3,
			wantAction:       ActionScaleUp,
			wantDeltaSign:    1,
		},
		{
			name: "scale up capped at max instances",
			status: taskqueue.QueueStatus{
				Pending: 10,
				Running: 1,
				Total:   15,
			},
			currentInstances: 6,
			options:          []Option{WithMaxInstances(8)},
			wantAction:       ActionScaleUp,
			wantDeltaSign:    1,
		},
		{
			name: "no scale up when already at max",
			status: taskqueue.QueueStatus{
				Pending: 5,
				Running: 2,
				Total:   10,
			},
			currentInstances: 8,
			options:          []Option{WithMaxInstances(8)},
			wantAction:       ActionNone,
			wantDeltaSign:    0,
		},
		{
			name: "no scale up when pending <= running",
			status: taskqueue.QueueStatus{
				Pending: 2,
				Running: 3,
				Total:   10,
			},
			currentInstances: 3,
			wantAction:       ActionNone,
			wantDeltaSign:    0,
		},
		{
			name: "scale down when idle",
			status: taskqueue.QueueStatus{
				Pending: 0,
				Running: 0,
				Total:   10,
			},
			currentInstances: 4,
			options:          []Option{WithMinInstances(1), WithScaleDownThreshold(1)},
			wantAction:       ActionScaleDown,
			wantDeltaSign:    -1,
		},
		{
			name: "scale down when running below threshold",
			status: taskqueue.QueueStatus{
				Pending: 0,
				Running: 1,
				Total:   10,
			},
			currentInstances: 3,
			options:          []Option{WithMinInstances(1), WithScaleDownThreshold(1)},
			wantAction:       ActionScaleDown,
			wantDeltaSign:    -1,
		},
		{
			name: "no scale down when at min",
			status: taskqueue.QueueStatus{
				Pending: 0,
				Running: 0,
				Total:   10,
			},
			currentInstances: 1,
			options:          []Option{WithMinInstances(1)},
			wantAction:       ActionNone,
			wantDeltaSign:    0,
		},
		{
			name: "no scale down when running above threshold",
			status: taskqueue.QueueStatus{
				Pending: 0,
				Running: 3,
				Total:   10,
			},
			currentInstances: 4,
			options:          []Option{WithScaleDownThreshold(1)},
			wantAction:       ActionNone,
			wantDeltaSign:    0,
		},
		{
			name: "no scaling when balanced",
			status: taskqueue.QueueStatus{
				Pending: 0,
				Running: 3,
				Total:   10,
			},
			currentInstances: 3,
			wantAction:       ActionNone,
			wantDeltaSign:    0,
		},
		{
			name: "no pending means no scale up even with zero running",
			status: taskqueue.QueueStatus{
				Pending:   0,
				Running:   0,
				Completed: 10,
				Total:     10,
			},
			currentInstances: 1,
			options:          []Option{WithMinInstances(1)},
			wantAction:       ActionNone,
			wantDeltaSign:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := append([]Option{WithCooldownPeriod(0)}, tt.options...)
			p := NewPolicy(opts...)
			d := p.Evaluate(tt.status, tt.currentInstances)

			if d.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", d.Action, tt.wantAction)
			}

			switch tt.wantDeltaSign {
			case 1:
				if d.Delta <= 0 {
					t.Errorf("Delta = %d, want positive", d.Delta)
				}
			case -1:
				if d.Delta >= 0 {
					t.Errorf("Delta = %d, want negative", d.Delta)
				}
			case 0:
				if d.Delta != 0 {
					t.Errorf("Delta = %d, want 0", d.Delta)
				}
			}

			if d.Reason == "" {
				t.Error("Reason should not be empty")
			}
		})
	}
}

func TestPolicy_Evaluate_Cooldown(t *testing.T) {
	p := NewPolicy(
		WithCooldownPeriod(time.Hour), // Long cooldown
	)

	status := taskqueue.QueueStatus{
		Pending: 5,
		Running: 1,
		Total:   10,
	}

	// First call should succeed
	d1 := p.Evaluate(status, 2)
	if d1.Action != ActionScaleUp {
		t.Fatalf("first call: Action = %q, want scale_up", d1.Action)
	}

	// Second call should be blocked by cooldown
	d2 := p.Evaluate(status, 2)
	if d2.Action != ActionNone {
		t.Errorf("second call: Action = %q, want none (cooldown)", d2.Action)
	}
	if d2.Reason != "cooldown period active" {
		t.Errorf("Reason = %q, want 'cooldown period active'", d2.Reason)
	}
}

func TestPolicy_Evaluate_ScaleUpDeltaCapped(t *testing.T) {
	p := NewPolicy(
		WithMaxInstances(5),
		WithCooldownPeriod(0),
	)

	status := taskqueue.QueueStatus{
		Pending: 10,
		Running: 1,
		Total:   15,
	}

	// Currently at 3, max is 5, so delta should be capped at 2
	d := p.Evaluate(status, 3)
	if d.Action != ActionScaleUp {
		t.Fatalf("Action = %q, want scale_up", d.Action)
	}
	if d.Delta != 2 {
		t.Errorf("Delta = %d, want 2 (capped at max)", d.Delta)
	}
}

func TestPolicy_Evaluate_ScaleDownByOne(t *testing.T) {
	p := NewPolicy(
		WithMinInstances(1),
		WithScaleDownThreshold(1),
		WithCooldownPeriod(0),
	)

	status := taskqueue.QueueStatus{
		Pending: 0,
		Running: 0,
		Total:   10,
	}

	// Scale down from 5 should only remove 1 at a time
	d := p.Evaluate(status, 5)
	if d.Action != ActionScaleDown {
		t.Fatalf("Action = %q, want scale_down", d.Action)
	}
	if d.Delta != -1 {
		t.Errorf("Delta = %d, want -1", d.Delta)
	}
}

func TestAction_String(t *testing.T) {
	tests := []struct {
		action Action
		want   string
	}{
		{ActionScaleUp, "scale_up"},
		{ActionScaleDown, "scale_down"},
		{ActionNone, "none"},
	}
	for _, tt := range tests {
		if got := tt.action.String(); got != tt.want {
			t.Errorf("Action(%q).String() = %q, want %q", tt.action, got, tt.want)
		}
	}
}
