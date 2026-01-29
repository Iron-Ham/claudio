package tui

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/tui/view"
)

func TestCollapseAdversarialRound(t *testing.T) {
	// Note: collapseAdversarialRound now collapses the "Previous Rounds" container
	// instead of individual round sub-groups. The container ID is based on the
	// session's GroupID (or session ID if GroupID is empty).
	tests := []struct {
		name                  string
		session               *adversarial.Session
		round                 int
		initialGroupViewState *view.GroupViewState
		wantCollapsed         bool
		wantContainerID       string
	}{
		{
			name:          "nil session",
			session:       nil,
			round:         1,
			wantCollapsed: false,
		},
		{
			name: "valid round collapses previous rounds container",
			session: &adversarial.Session{
				ID: "test-session",
				History: []adversarial.Round{
					{
						Round:      1,
						SubGroupID: "test-session-round-1",
						StartedAt:  time.Now(),
					},
				},
			},
			round:           1,
			wantCollapsed:   true,
			wantContainerID: "test-session-previous-rounds",
		},
		{
			name: "valid round with GroupID uses GroupID prefix",
			session: &adversarial.Session{
				ID:      "test-session",
				GroupID: "adv-group-123",
				History: []adversarial.Round{
					{
						Round:      1,
						SubGroupID: "adv-group-123-round-1",
						StartedAt:  time.Now(),
					},
				},
			},
			round:           1,
			wantCollapsed:   true,
			wantContainerID: "adv-group-123-previous-rounds",
		},
		{
			name: "valid round with existing groupViewState",
			session: &adversarial.Session{
				ID: "test-session",
				History: []adversarial.Round{
					{
						Round:      1,
						SubGroupID: "test-session-round-1",
						StartedAt:  time.Now(),
					},
				},
			},
			round:                 1,
			initialGroupViewState: view.NewGroupViewState(),
			wantCollapsed:         true,
			wantContainerID:       "test-session-previous-rounds",
		},
		{
			name: "round number too low does not collapse",
			session: &adversarial.Session{
				ID: "test-session",
				History: []adversarial.Round{
					{Round: 1, SubGroupID: "test-session-round-1"},
				},
			},
			round:         0, // Invalid round number
			wantCollapsed: false,
		},
		{
			name: "multiple rounds collapse previous rounds container",
			session: &adversarial.Session{
				ID: "test-session",
				History: []adversarial.Round{
					{Round: 1, SubGroupID: "test-session-round-1"},
					{Round: 2, SubGroupID: "test-session-round-2"},
					{Round: 3, SubGroupID: "test-session-round-3"},
				},
			},
			round:           2,
			wantCollapsed:   true,
			wantContainerID: "test-session-previous-rounds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				groupViewState: tt.initialGroupViewState,
			}

			m.collapseAdversarialRound(tt.session, tt.round)

			if tt.wantCollapsed {
				if m.groupViewState == nil {
					t.Fatal("groupViewState should have been initialized")
				}
				if !m.groupViewState.CollapsedGroups[tt.wantContainerID] {
					t.Errorf("expected container %q to be collapsed, got collapsed groups: %v",
						tt.wantContainerID, m.groupViewState.CollapsedGroups)
				}
			} else if tt.wantContainerID != "" {
				// If we expected no collapse but had a container ID, verify it's not collapsed
				if m.groupViewState != nil && m.groupViewState.CollapsedGroups[tt.wantContainerID] {
					t.Errorf("expected container %q to NOT be collapsed", tt.wantContainerID)
				}
			}
		})
	}
}

func TestCollapseAdversarialRoundInitializesGroupViewState(t *testing.T) {
	m := &Model{
		groupViewState: nil, // Start with nil
	}

	session := &adversarial.Session{
		ID: "test-session",
		History: []adversarial.Round{
			{Round: 1, SubGroupID: "test-session-round-1"},
		},
	}

	m.collapseAdversarialRound(session, 1)

	if m.groupViewState == nil {
		t.Fatal("groupViewState should have been initialized")
	}
	if m.groupViewState.CollapsedGroups == nil {
		t.Fatal("CollapsedGroups map should have been initialized")
	}
	// Should collapse the "Previous Rounds" container
	expectedContainerID := "test-session-previous-rounds"
	if !m.groupViewState.CollapsedGroups[expectedContainerID] {
		t.Errorf("expected container %q to be collapsed", expectedContainerID)
	}
}

func TestCollapseAdversarialRoundPreservesExistingState(t *testing.T) {
	// Pre-populate with existing collapsed group
	existingState := view.NewGroupViewState()
	existingState.CollapsedGroups["other-group"] = true

	m := &Model{
		groupViewState: existingState,
	}

	session := &adversarial.Session{
		ID: "test-session",
		History: []adversarial.Round{
			{Round: 1, SubGroupID: "test-session-round-1"},
		},
	}

	m.collapseAdversarialRound(session, 1)

	// Verify both groups are collapsed
	if !m.groupViewState.CollapsedGroups["other-group"] {
		t.Error("existing collapsed group should be preserved")
	}
	expectedContainerID := "test-session-previous-rounds"
	if !m.groupViewState.CollapsedGroups[expectedContainerID] {
		t.Errorf("expected container %q to be collapsed", expectedContainerID)
	}
}
