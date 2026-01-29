package tui

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/tui/view"
)

func TestCollapseAdversarialRound(t *testing.T) {
	tests := []struct {
		name                  string
		session               *adversarial.Session
		round                 int
		initialGroupViewState *view.GroupViewState
		wantCollapsed         bool
		wantSubGroupID        string
	}{
		{
			name:          "nil session",
			session:       nil,
			round:         1,
			wantCollapsed: false,
		},
		{
			name: "valid round with sub-group ID",
			session: &adversarial.Session{
				ID: "test-session",
				History: []adversarial.Round{
					{
						Round:      1,
						SubGroupID: "adv-123-round-1",
						StartedAt:  time.Now(),
					},
				},
			},
			round:          1,
			wantCollapsed:  true,
			wantSubGroupID: "adv-123-round-1",
		},
		{
			name: "valid round with existing groupViewState",
			session: &adversarial.Session{
				ID: "test-session",
				History: []adversarial.Round{
					{
						Round:      1,
						SubGroupID: "adv-123-round-1",
						StartedAt:  time.Now(),
					},
				},
			},
			round:                 1,
			initialGroupViewState: view.NewGroupViewState(),
			wantCollapsed:         true,
			wantSubGroupID:        "adv-123-round-1",
		},
		{
			name: "round number too low",
			session: &adversarial.Session{
				ID: "test-session",
				History: []adversarial.Round{
					{Round: 1, SubGroupID: "adv-123-round-1"},
				},
			},
			round:         0, // Invalid round number
			wantCollapsed: false,
		},
		{
			name: "round number too high",
			session: &adversarial.Session{
				ID: "test-session",
				History: []adversarial.Round{
					{Round: 1, SubGroupID: "adv-123-round-1"},
				},
			},
			round:         5, // Higher than history length
			wantCollapsed: false,
		},
		{
			name: "empty sub-group ID in history",
			session: &adversarial.Session{
				ID: "test-session",
				History: []adversarial.Round{
					{Round: 1, SubGroupID: ""}, // Empty sub-group ID
				},
			},
			round:         1,
			wantCollapsed: false,
		},
		{
			name: "multiple rounds collapse second round",
			session: &adversarial.Session{
				ID: "test-session",
				History: []adversarial.Round{
					{Round: 1, SubGroupID: "adv-123-round-1"},
					{Round: 2, SubGroupID: "adv-123-round-2"},
					{Round: 3, SubGroupID: "adv-123-round-3"},
				},
			},
			round:          2,
			wantCollapsed:  true,
			wantSubGroupID: "adv-123-round-2",
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
				if !m.groupViewState.CollapsedGroups[tt.wantSubGroupID] {
					t.Errorf("expected sub-group %q to be collapsed", tt.wantSubGroupID)
				}
			} else if tt.wantSubGroupID != "" {
				// If we expected no collapse but had a sub-group ID, verify it's not collapsed
				if m.groupViewState != nil && m.groupViewState.CollapsedGroups[tt.wantSubGroupID] {
					t.Errorf("expected sub-group %q to NOT be collapsed", tt.wantSubGroupID)
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
			{Round: 1, SubGroupID: "adv-123-round-1"},
		},
	}

	m.collapseAdversarialRound(session, 1)

	if m.groupViewState == nil {
		t.Fatal("groupViewState should have been initialized")
	}
	if m.groupViewState.CollapsedGroups == nil {
		t.Fatal("CollapsedGroups map should have been initialized")
	}
	if !m.groupViewState.CollapsedGroups["adv-123-round-1"] {
		t.Error("expected sub-group to be collapsed")
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
			{Round: 1, SubGroupID: "adv-123-round-1"},
		},
	}

	m.collapseAdversarialRound(session, 1)

	// Verify both groups are collapsed
	if !m.groupViewState.CollapsedGroups["other-group"] {
		t.Error("existing collapsed group should be preserved")
	}
	if !m.groupViewState.CollapsedGroups["adv-123-round-1"] {
		t.Error("new group should be collapsed")
	}
}
