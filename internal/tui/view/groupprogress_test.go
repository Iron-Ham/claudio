package view

import (
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestCalculateGroupMetrics(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-30 * time.Minute)
	later := now.Add(-10 * time.Minute)

	tests := []struct {
		name     string
		group    *orchestrator.InstanceGroup
		session  *orchestrator.Session
		expected *GroupMetrics
	}{
		{
			name:     "nil group returns nil",
			group:    nil,
			session:  &orchestrator.Session{},
			expected: nil,
		},
		{
			name:     "nil session returns nil",
			group:    &orchestrator.InstanceGroup{ID: "g1"},
			session:  nil,
			expected: nil,
		},
		{
			name: "empty group",
			group: &orchestrator.InstanceGroup{
				ID:        "g1",
				Name:      "Test Group",
				Phase:     orchestrator.GroupPhasePending,
				Instances: []string{},
			},
			session: &orchestrator.Session{},
			expected: &GroupMetrics{
				GroupID:   "g1",
				GroupName: "Test Group",
				Phase:     orchestrator.GroupPhasePending,
				Total:     0,
				Completed: 0,
			},
		},
		{
			name: "group with completed instances",
			group: &orchestrator.InstanceGroup{
				ID:        "g1",
				Name:      "Test Group",
				Phase:     orchestrator.GroupPhaseCompleted,
				Instances: []string{"inst1", "inst2"},
			},
			session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{
						ID:     "inst1",
						Status: orchestrator.StatusCompleted,
						Metrics: &orchestrator.Metrics{
							InputTokens:  1000,
							OutputTokens: 500,
							Cost:         0.25,
							APICalls:     5,
							StartTime:    &earlier,
							EndTime:      &later,
						},
					},
					{
						ID:     "inst2",
						Status: orchestrator.StatusCompleted,
						Metrics: &orchestrator.Metrics{
							InputTokens:  2000,
							OutputTokens: 1000,
							Cost:         0.50,
							APICalls:     10,
							StartTime:    &earlier,
							EndTime:      &later,
						},
					},
				},
			},
			expected: &GroupMetrics{
				GroupID:      "g1",
				GroupName:    "Test Group",
				Phase:        orchestrator.GroupPhaseCompleted,
				Total:        2,
				Completed:    2,
				InputTokens:  3000,
				OutputTokens: 1500,
				Cost:         0.75,
				APICalls:     15,
			},
		},
		{
			name: "group with mixed status instances",
			group: &orchestrator.InstanceGroup{
				ID:        "g1",
				Name:      "Mixed Group",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst1", "inst2", "inst3"},
			},
			session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{
						ID:     "inst1",
						Status: orchestrator.StatusCompleted,
						Metrics: &orchestrator.Metrics{
							InputTokens:  1000,
							OutputTokens: 500,
							Cost:         0.25,
						},
					},
					{
						ID:     "inst2",
						Status: orchestrator.StatusWorking,
						Metrics: &orchestrator.Metrics{
							InputTokens:  500,
							OutputTokens: 250,
							Cost:         0.10,
						},
					},
					{
						ID:      "inst3",
						Status:  orchestrator.StatusPending,
						Metrics: nil, // No metrics yet
					},
				},
			},
			expected: &GroupMetrics{
				GroupID:      "g1",
				GroupName:    "Mixed Group",
				Phase:        orchestrator.GroupPhaseExecuting,
				Total:        3,
				Completed:    1,
				InputTokens:  1500,
				OutputTokens: 750,
				Cost:         0.35,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateGroupMetrics(tt.group, tt.session)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.GroupID != tt.expected.GroupID {
				t.Errorf("GroupID = %s, want %s", result.GroupID, tt.expected.GroupID)
			}
			if result.GroupName != tt.expected.GroupName {
				t.Errorf("GroupName = %s, want %s", result.GroupName, tt.expected.GroupName)
			}
			if result.Phase != tt.expected.Phase {
				t.Errorf("Phase = %s, want %s", result.Phase, tt.expected.Phase)
			}
			if result.Total != tt.expected.Total {
				t.Errorf("Total = %d, want %d", result.Total, tt.expected.Total)
			}
			if result.Completed != tt.expected.Completed {
				t.Errorf("Completed = %d, want %d", result.Completed, tt.expected.Completed)
			}
			if result.InputTokens != tt.expected.InputTokens {
				t.Errorf("InputTokens = %d, want %d", result.InputTokens, tt.expected.InputTokens)
			}
			if result.OutputTokens != tt.expected.OutputTokens {
				t.Errorf("OutputTokens = %d, want %d", result.OutputTokens, tt.expected.OutputTokens)
			}
			if result.Cost != tt.expected.Cost {
				t.Errorf("Cost = %f, want %f", result.Cost, tt.expected.Cost)
			}
		})
	}
}

func TestCalculateSessionGroupMetrics(t *testing.T) {
	tests := []struct {
		name     string
		session  *orchestrator.Session
		expected *SessionGroupMetrics
	}{
		{
			name:     "nil session returns nil",
			session:  nil,
			expected: nil,
		},
		{
			name: "session with no groups returns nil",
			session: &orchestrator.Session{
				Groups: []*orchestrator.InstanceGroup{},
			},
			expected: nil,
		},
		{
			name: "session with groups",
			session: &orchestrator.Session{
				Groups: []*orchestrator.InstanceGroup{
					{
						ID:        "g1",
						Name:      "Group 1",
						Phase:     orchestrator.GroupPhaseCompleted,
						Instances: []string{"inst1"},
					},
					{
						ID:        "g2",
						Name:      "Group 2",
						Phase:     orchestrator.GroupPhaseExecuting,
						Instances: []string{"inst2", "inst3"},
					},
				},
				Instances: []*orchestrator.Instance{
					{
						ID:     "inst1",
						Status: orchestrator.StatusCompleted,
						Metrics: &orchestrator.Metrics{
							InputTokens:  1000,
							OutputTokens: 500,
							Cost:         0.25,
						},
					},
					{
						ID:     "inst2",
						Status: orchestrator.StatusCompleted,
						Metrics: &orchestrator.Metrics{
							InputTokens:  2000,
							OutputTokens: 1000,
							Cost:         0.50,
						},
					},
					{
						ID:     "inst3",
						Status: orchestrator.StatusWorking,
						Metrics: &orchestrator.Metrics{
							InputTokens:  500,
							OutputTokens: 250,
							Cost:         0.10,
						},
					},
				},
			},
			expected: &SessionGroupMetrics{
				TotalCompleted: 2,
				TotalInstances: 3,
				TotalCost:      0.85,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSessionGroupMetrics(tt.session)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.TotalCompleted != tt.expected.TotalCompleted {
				t.Errorf("TotalCompleted = %d, want %d", result.TotalCompleted, tt.expected.TotalCompleted)
			}
			if result.TotalInstances != tt.expected.TotalInstances {
				t.Errorf("TotalInstances = %d, want %d", result.TotalInstances, tt.expected.TotalInstances)
			}
			if result.TotalCost != tt.expected.TotalCost {
				t.Errorf("TotalCost = %f, want %f", result.TotalCost, tt.expected.TotalCost)
			}
		})
	}
}

func TestRenderOverallProgressBar(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		total     int
		width     int
		expected  string
	}{
		{
			name:      "0 percent",
			completed: 0,
			total:     10,
			width:     10,
			expected:  "[░░░░░░░░░░] 0%",
		},
		{
			name:      "50 percent",
			completed: 5,
			total:     10,
			width:     10,
			expected:  "[█████░░░░░] 50%",
		},
		{
			name:      "100 percent",
			completed: 10,
			total:     10,
			width:     10,
			expected:  "[██████████] 100%",
		},
		{
			name:      "zero total",
			completed: 0,
			total:     0,
			width:     10,
			expected:  "[░░░░░░░░░░]",
		},
		{
			name:      "partial progress",
			completed: 3,
			total:     10,
			width:     10,
			expected:  "[███░░░░░░░] 30%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderOverallProgressBar(tt.completed, tt.total, tt.width)
			if result != tt.expected {
				t.Errorf("RenderOverallProgressBar(%d, %d, %d) = %q, want %q",
					tt.completed, tt.total, tt.width, result, tt.expected)
			}
		})
	}
}

func TestRenderMiniProgressBar(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		total     int
		barWidth  int
		expected  string
	}{
		{
			name:      "0 of 5",
			completed: 0,
			total:     5,
			barWidth:  5,
			expected:  "[░░░░░] 0/5",
		},
		{
			name:      "3 of 5",
			completed: 3,
			total:     5,
			barWidth:  5,
			expected:  "[███░░] 3/5",
		},
		{
			name:      "5 of 5",
			completed: 5,
			total:     5,
			barWidth:  5,
			expected:  "[█████] 5/5",
		},
		{
			name:      "zero total",
			completed: 0,
			total:     0,
			barWidth:  5,
			expected:  "[░░░░░] 0/0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderMiniProgressBar(tt.completed, tt.total, tt.barWidth)
			if result != tt.expected {
				t.Errorf("RenderMiniProgressBar(%d, %d, %d) = %q, want %q",
					tt.completed, tt.total, tt.barWidth, result, tt.expected)
			}
		})
	}
}

func TestRenderGroupMetricsCompact(t *testing.T) {
	tests := []struct {
		name     string
		metrics  *GroupMetrics
		expected string
	}{
		{
			name:     "nil metrics",
			metrics:  nil,
			expected: "",
		},
		{
			name: "empty metrics",
			metrics: &GroupMetrics{
				InputTokens:  0,
				OutputTokens: 0,
				Cost:         0,
				Duration:     0,
			},
			expected: "",
		},
		{
			name: "tokens only",
			metrics: &GroupMetrics{
				InputTokens:  1000,
				OutputTokens: 500,
				Cost:         0,
				Duration:     0,
			},
			expected: "1.5K tok",
		},
		{
			name: "cost only",
			metrics: &GroupMetrics{
				InputTokens:  0,
				OutputTokens: 0,
				Cost:         0.42,
				Duration:     0,
			},
			expected: "$0.42",
		},
		{
			name: "all metrics",
			metrics: &GroupMetrics{
				InputTokens:  45000,
				OutputTokens: 12000,
				Cost:         1.23,
				Duration:     2*time.Minute + 30*time.Second,
			},
			expected: "57.0K tok | $1.23 | 2m 30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderGroupMetricsCompact(tt.metrics)
			if result != tt.expected {
				t.Errorf("RenderGroupMetricsCompact() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRenderGroupProgressFooter(t *testing.T) {
	tests := []struct {
		name           string
		session        *orchestrator.Session
		expectedPrefix string
	}{
		{
			name:           "nil session returns empty",
			session:        nil,
			expectedPrefix: "",
		},
		{
			name: "no groups returns empty",
			session: &orchestrator.Session{
				Groups: []*orchestrator.InstanceGroup{},
			},
			expectedPrefix: "",
		},
		{
			name: "session with groups",
			session: &orchestrator.Session{
				Groups: []*orchestrator.InstanceGroup{
					{
						ID:        "g1",
						Name:      "Group 1",
						Instances: []string{"inst1", "inst2"},
					},
				},
				Instances: []*orchestrator.Instance{
					{ID: "inst1", Status: orchestrator.StatusCompleted},
					{ID: "inst2", Status: orchestrator.StatusWorking},
				},
			},
			expectedPrefix: "Groups:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderGroupProgressFooter(tt.session)
			if tt.expectedPrefix == "" {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
			} else {
				if !strings.HasPrefix(result, tt.expectedPrefix) {
					t.Errorf("expected prefix %q, got %q", tt.expectedPrefix, result)
				}
			}
		})
	}
}

func TestGroupProgressViewRender(t *testing.T) {
	tests := []struct {
		name             string
		session          *orchestrator.Session
		width            int
		expectedContains []string
	}{
		{
			name:             "nil session",
			session:          nil,
			width:            80,
			expectedContains: []string{"No groups configured"},
		},
		{
			name: "session with groups",
			session: &orchestrator.Session{
				Groups: []*orchestrator.InstanceGroup{
					{
						ID:        "g1",
						Name:      "Group 1: Setup",
						Phase:     orchestrator.GroupPhaseCompleted,
						Instances: []string{"inst1"},
					},
					{
						ID:        "g2",
						Name:      "Group 2: Build",
						Phase:     orchestrator.GroupPhaseExecuting,
						Instances: []string{"inst2", "inst3"},
					},
				},
				Instances: []*orchestrator.Instance{
					{ID: "inst1", Status: orchestrator.StatusCompleted},
					{ID: "inst2", Status: orchestrator.StatusCompleted},
					{ID: "inst3", Status: orchestrator.StatusWorking},
				},
			},
			width: 80,
			expectedContains: []string{
				"Group Progress",
				"Overall:",
				"Group 1",
				"Group 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := NewGroupProgressView(tt.width)
			result := view.Render(tt.session)

			for _, expected := range tt.expectedContains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected result to contain %q, got:\n%s", expected, result)
				}
			}
		})
	}
}

func TestRenderGroupStatsPanel(t *testing.T) {
	tests := []struct {
		name             string
		session          *orchestrator.Session
		width            int
		expectedContains []string
	}{
		{
			name:             "nil session",
			session:          nil,
			width:            80,
			expectedContains: []string{"No groups configured"},
		},
		{
			name: "session with metrics",
			session: &orchestrator.Session{
				Groups: []*orchestrator.InstanceGroup{
					{
						ID:        "g1",
						Name:      "Test Group",
						Phase:     orchestrator.GroupPhaseCompleted,
						Instances: []string{"inst1"},
					},
				},
				Instances: []*orchestrator.Instance{
					{
						ID:     "inst1",
						Status: orchestrator.StatusCompleted,
						Metrics: &orchestrator.Metrics{
							InputTokens:  10000,
							OutputTokens: 5000,
							Cost:         0.50,
						},
					},
				},
			},
			width: 80,
			expectedContains: []string{
				"Group Statistics",
				"Overall Progress",
				"Per-Group Breakdown",
				"Test Group",
				"Progress:",
				"Tokens:",
				"Cost:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderGroupStatsPanel(tt.session, tt.width)

			for _, expected := range tt.expectedContains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected result to contain %q, got:\n%s", expected, result)
				}
			}
		})
	}
}

func TestCalculateGroupMetricsWithSubGroups(t *testing.T) {
	// Test that sub-groups are properly included in metrics calculation
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{
				ID:     "inst1",
				Status: orchestrator.StatusCompleted,
				Metrics: &orchestrator.Metrics{
					InputTokens:  1000,
					OutputTokens: 500,
					Cost:         0.25,
				},
			},
			{
				ID:     "inst2",
				Status: orchestrator.StatusCompleted,
				Metrics: &orchestrator.Metrics{
					InputTokens:  2000,
					OutputTokens: 1000,
					Cost:         0.50,
				},
			},
			{
				ID:     "inst3",
				Status: orchestrator.StatusWorking,
				Metrics: &orchestrator.Metrics{
					InputTokens:  500,
					OutputTokens: 250,
					Cost:         0.10,
				},
			},
		},
	}

	subGroup := &orchestrator.InstanceGroup{
		ID:        "sg1",
		Name:      "Sub Group",
		Phase:     orchestrator.GroupPhaseExecuting,
		Instances: []string{"inst3"},
	}

	group := &orchestrator.InstanceGroup{
		ID:        "g1",
		Name:      "Parent Group",
		Phase:     orchestrator.GroupPhaseExecuting,
		Instances: []string{"inst1", "inst2"},
		SubGroups: []*orchestrator.InstanceGroup{subGroup},
	}

	result := CalculateGroupMetrics(group, session)

	// Total should include instances from sub-group
	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}

	// Completed should be 2 (inst1 and inst2 are completed, inst3 is working)
	if result.Completed != 2 {
		t.Errorf("Completed = %d, want 2", result.Completed)
	}

	// Tokens should be summed from all instances
	expectedInput := int64(3500) // 1000 + 2000 + 500
	if result.InputTokens != expectedInput {
		t.Errorf("InputTokens = %d, want %d", result.InputTokens, expectedInput)
	}

	expectedOutput := int64(1750) // 500 + 1000 + 250
	if result.OutputTokens != expectedOutput {
		t.Errorf("OutputTokens = %d, want %d", result.OutputTokens, expectedOutput)
	}

	// Cost should be summed
	expectedCost := 0.85 // 0.25 + 0.50 + 0.10
	if result.Cost != expectedCost {
		t.Errorf("Cost = %f, want %f", result.Cost, expectedCost)
	}
}

func TestETAEstimation(t *testing.T) {
	now := time.Now()
	start1 := now.Add(-10 * time.Minute)
	end1 := now.Add(-5 * time.Minute)
	start2 := now.Add(-8 * time.Minute)
	end2 := now.Add(-3 * time.Minute)

	session := &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "g1",
				Name:      "Group 1",
				Phase:     orchestrator.GroupPhaseCompleted,
				Instances: []string{"inst1", "inst2"},
			},
			{
				ID:        "g2",
				Name:      "Group 2",
				Phase:     orchestrator.GroupPhasePending,
				Instances: []string{"inst3", "inst4"},
			},
		},
		Instances: []*orchestrator.Instance{
			{
				ID:     "inst1",
				Status: orchestrator.StatusCompleted,
				Metrics: &orchestrator.Metrics{
					StartTime: &start1,
					EndTime:   &end1, // 5 minute duration
				},
			},
			{
				ID:     "inst2",
				Status: orchestrator.StatusCompleted,
				Metrics: &orchestrator.Metrics{
					StartTime: &start2,
					EndTime:   &end2, // 5 minute duration
				},
			},
			{
				ID:     "inst3",
				Status: orchestrator.StatusPending,
			},
			{
				ID:     "inst4",
				Status: orchestrator.StatusPending,
			},
		},
	}

	result := CalculateSessionGroupMetrics(session)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// 2 completed, 4 total, so 2 remaining
	// Average duration = 5 minutes per instance
	// ETA should be approximately 10 minutes (2 * 5 minutes)
	if result.EstimatedETA == 0 {
		t.Error("expected non-zero ETA")
	}

	// Allow some tolerance in the estimate
	expectedETA := 10 * time.Minute
	tolerance := 1 * time.Minute
	if result.EstimatedETA < expectedETA-tolerance || result.EstimatedETA > expectedETA+tolerance {
		t.Errorf("EstimatedETA = %v, expected approximately %v", result.EstimatedETA, expectedETA)
	}
}
