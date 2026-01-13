package view

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestRenderGroupedModeFooter(t *testing.T) {
	tests := []struct {
		name             string
		state            FooterState
		expectEmpty      bool
		expectedContains []string
	}{
		{
			name: "returns empty when grouped view disabled",
			state: FooterState{
				Session: &orchestrator.Session{
					Groups: []*orchestrator.InstanceGroup{
						{ID: "g1", Name: "Group 1", Instances: []string{"i1"}},
					},
				},
				GroupedViewEnabled: false,
				Width:              80,
			},
			expectEmpty: true,
		},
		{
			name: "returns empty when session is nil",
			state: FooterState{
				Session:            nil,
				GroupedViewEnabled: true,
				Width:              80,
			},
			expectEmpty: true,
		},
		{
			name: "returns empty when no groups",
			state: FooterState{
				Session:            &orchestrator.Session{Groups: []*orchestrator.InstanceGroup{}},
				GroupedViewEnabled: true,
				Width:              80,
			},
			expectEmpty: true,
		},
		{
			name: "renders progress for session with groups",
			state: FooterState{
				Session: &orchestrator.Session{
					Groups: []*orchestrator.InstanceGroup{
						{ID: "g1", Name: "Group 1", Phase: orchestrator.GroupPhaseCompleted, Instances: []string{"i1"}},
						{ID: "g2", Name: "Group 2", Phase: orchestrator.GroupPhaseExecuting, Instances: []string{"i2", "i3"}},
					},
					Instances: []*orchestrator.Instance{
						{ID: "i1", Status: orchestrator.StatusCompleted},
						{ID: "i2", Status: orchestrator.StatusCompleted},
						{ID: "i3", Status: orchestrator.StatusWorking},
					},
				},
				GroupedViewEnabled: true,
				Width:              80,
			},
			expectEmpty:      false,
			expectedContains: []string{"Groups:", "["},
		},
		{
			name: "includes cost when present",
			state: FooterState{
				Session: &orchestrator.Session{
					Groups: []*orchestrator.InstanceGroup{
						{ID: "g1", Name: "Group 1", Instances: []string{"i1"}},
					},
					Instances: []*orchestrator.Instance{
						{
							ID:     "i1",
							Status: orchestrator.StatusCompleted,
							Metrics: &orchestrator.Metrics{
								Cost: 0.50,
							},
						},
					},
				},
				GroupedViewEnabled: true,
				Width:              80,
			},
			expectEmpty:      false,
			expectedContains: []string{"$0.50"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderGroupedModeFooter(tt.state)

			if tt.expectEmpty {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
				return
			}

			if result == "" {
				t.Error("expected non-empty result")
				return
			}

			for _, expected := range tt.expectedContains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected result to contain %q, got %q", expected, result)
				}
			}
		})
	}
}

func TestRenderGroupPhaseSummary(t *testing.T) {
	tests := []struct {
		name             string
		metrics          *SessionGroupMetrics
		expectEmpty      bool
		expectedContains []string
	}{
		{
			name:        "nil metrics returns empty",
			metrics:     nil,
			expectEmpty: true,
		},
		{
			name: "empty groups returns empty",
			metrics: &SessionGroupMetrics{
				Groups: []*GroupMetrics{},
			},
			expectEmpty: true,
		},
		{
			name: "renders phase counts",
			metrics: &SessionGroupMetrics{
				Groups: []*GroupMetrics{
					{Phase: orchestrator.GroupPhaseCompleted},
					{Phase: orchestrator.GroupPhaseCompleted},
					{Phase: orchestrator.GroupPhaseExecuting},
					{Phase: orchestrator.GroupPhasePending},
					{Phase: orchestrator.GroupPhasePending},
					{Phase: orchestrator.GroupPhasePending},
				},
			},
			expectEmpty:      false,
			expectedContains: []string{"✓2", "●1", "○3"},
		},
		{
			name: "renders failed groups",
			metrics: &SessionGroupMetrics{
				Groups: []*GroupMetrics{
					{Phase: orchestrator.GroupPhaseFailed},
					{Phase: orchestrator.GroupPhaseCompleted},
				},
			},
			expectEmpty:      false,
			expectedContains: []string{"✗1", "✓1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderGroupPhaseSummary(tt.metrics)

			if tt.expectEmpty {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
				return
			}

			for _, expected := range tt.expectedContains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected result to contain %q, got %q", expected, result)
				}
			}
		})
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{99, "99"},
		{100, "100"},
		{999, "999"},
		{1234, "1234"},
		{-1, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatInt(tt.input)
			if result != tt.expected {
				t.Errorf("formatInt(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "000%"},
		{1, "001%"},
		{10, "010%"},
		{50, "050%"},
		{99, "099%"},
		{100, "100%"},
		{-5, "0%"},
		{150, "100%"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatPercent(tt.input)
			if result != tt.expected {
				t.Errorf("formatPercent(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRenderGroupNavigationHint(t *testing.T) {
	result := RenderGroupNavigationHint()

	expectedContains := []string{"J/K", "j/k", "Space"}
	for _, expected := range expectedContains {
		if !strings.Contains(result, expected) {
			t.Errorf("expected result to contain %q, got %q", expected, result)
		}
	}
}

func TestRenderGroupProgressBanner(t *testing.T) {
	tests := []struct {
		name             string
		session          *orchestrator.Session
		width            int
		expectEmpty      bool
		expectedContains []string
	}{
		{
			name:        "nil session returns empty",
			session:     nil,
			width:       80,
			expectEmpty: true,
		},
		{
			name: "renders progress banner",
			session: &orchestrator.Session{
				Groups: []*orchestrator.InstanceGroup{
					{ID: "g1", Name: "Group 1", Instances: []string{"i1", "i2"}},
				},
				Instances: []*orchestrator.Instance{
					{ID: "i1", Status: orchestrator.StatusCompleted},
					{ID: "i2", Status: orchestrator.StatusWorking},
				},
			},
			width:            80,
			expectEmpty:      false,
			expectedContains: []string{"Progress:", "["},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderGroupProgressBanner(tt.session, tt.width)

			if tt.expectEmpty {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
				return
			}

			for _, expected := range tt.expectedContains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected result to contain %q, got %q", expected, result)
				}
			}
		})
	}
}

// mockFooterProvider implements FooterProvider for testing
type mockFooterProvider struct {
	session            *orchestrator.Session
	groupedViewEnabled bool
}

func (m *mockFooterProvider) GetSession() *orchestrator.Session {
	return m.session
}

func (m *mockFooterProvider) IsGroupedViewEnabled() bool {
	return m.groupedViewEnabled
}

func TestRenderGroupedFooterFromProvider(t *testing.T) {
	tests := []struct {
		name        string
		provider    FooterProvider
		width       int
		expectEmpty bool
	}{
		{
			name:        "nil provider returns empty",
			provider:    nil,
			width:       80,
			expectEmpty: true,
		},
		{
			name: "disabled grouped view returns empty",
			provider: &mockFooterProvider{
				session: &orchestrator.Session{
					Groups: []*orchestrator.InstanceGroup{
						{ID: "g1", Instances: []string{"i1"}},
					},
				},
				groupedViewEnabled: false,
			},
			width:       80,
			expectEmpty: true,
		},
		{
			name: "enabled grouped view renders footer",
			provider: &mockFooterProvider{
				session: &orchestrator.Session{
					Groups: []*orchestrator.InstanceGroup{
						{ID: "g1", Instances: []string{"i1"}},
					},
					Instances: []*orchestrator.Instance{
						{ID: "i1", Status: orchestrator.StatusCompleted},
					},
				},
				groupedViewEnabled: true,
			},
			width:       80,
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderGroupedFooterFromProvider(tt.provider, tt.width)

			if tt.expectEmpty && result != "" {
				t.Errorf("expected empty string, got %q", result)
			}
			if !tt.expectEmpty && result == "" {
				t.Error("expected non-empty result")
			}
		})
	}
}
