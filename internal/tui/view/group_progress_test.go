package view

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/ralph"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
)

func TestIsInstanceCompleted(t *testing.T) {
	tests := []struct {
		status   orchestrator.InstanceStatus
		expected bool
	}{
		{orchestrator.StatusPending, false},
		{orchestrator.StatusPreparing, false},
		{orchestrator.StatusWorking, false},
		{orchestrator.StatusWaitingInput, false},
		{orchestrator.StatusPaused, false},
		{orchestrator.StatusCreatingPR, false},
		{orchestrator.StatusCompleted, true},
		{orchestrator.StatusError, true},
		{orchestrator.StatusStuck, true},
		{orchestrator.StatusTimeout, true},
		{orchestrator.StatusInterrupted, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := isInstanceCompleted(tt.status)
			if got != tt.expected {
				t.Errorf("isInstanceCompleted(%q) = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}

func TestIsTripleShotAttemptCompleted(t *testing.T) {
	tests := []struct {
		status   tripleshot.AttemptStatus
		expected bool
	}{
		{tripleshot.AttemptStatusPending, false},
		{tripleshot.AttemptStatusWorking, false},
		{tripleshot.AttemptStatusUnderReview, false},
		{tripleshot.AttemptStatusCompleted, true},
		{tripleshot.AttemptStatusFailed, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := isTripleShotAttemptCompleted(tt.status)
			if got != tt.expected {
				t.Errorf("isTripleShotAttemptCompleted(%q) = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}

func TestCalculateTripleShotProgress(t *testing.T) {
	tests := []struct {
		name          string
		tsSession     *tripleshot.Session
		instances     []*orchestrator.Instance
		groupID       string
		wantCompleted int
		wantTotal     int
	}{
		{
			name: "no attempts started yet",
			tsSession: &tripleshot.Session{
				ID:      "ts-1",
				GroupID: "group-1",
				Phase:   tripleshot.PhaseWorking,
				Attempts: [3]tripleshot.Attempt{
					{InstanceID: "", Status: tripleshot.AttemptStatusPending},
					{InstanceID: "", Status: tripleshot.AttemptStatusPending},
					{InstanceID: "", Status: tripleshot.AttemptStatusPending},
				},
			},
			groupID:       "group-1",
			wantCompleted: 0,
			wantTotal:     0,
		},
		{
			name: "all three attempts working",
			tsSession: &tripleshot.Session{
				ID:      "ts-1",
				GroupID: "group-1",
				Phase:   tripleshot.PhaseWorking,
				Attempts: [3]tripleshot.Attempt{
					{InstanceID: "inst-1", Status: tripleshot.AttemptStatusWorking},
					{InstanceID: "inst-2", Status: tripleshot.AttemptStatusWorking},
					{InstanceID: "inst-3", Status: tripleshot.AttemptStatusWorking},
				},
			},
			instances: []*orchestrator.Instance{
				{ID: "inst-1", Status: orchestrator.StatusWorking},
				{ID: "inst-2", Status: orchestrator.StatusWorking},
				{ID: "inst-3", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 0,
			wantTotal:     3,
		},
		{
			name: "two attempts completed, one working",
			tsSession: &tripleshot.Session{
				ID:      "ts-1",
				GroupID: "group-1",
				Phase:   tripleshot.PhaseWorking,
				Attempts: [3]tripleshot.Attempt{
					{InstanceID: "inst-1", Status: tripleshot.AttemptStatusCompleted},
					{InstanceID: "inst-2", Status: tripleshot.AttemptStatusCompleted},
					{InstanceID: "inst-3", Status: tripleshot.AttemptStatusWorking},
				},
			},
			instances: []*orchestrator.Instance{
				{ID: "inst-1", Status: orchestrator.StatusWorking}, // Instance status differs from attempt status
				{ID: "inst-2", Status: orchestrator.StatusWorking},
				{ID: "inst-3", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 2,
			wantTotal:     3,
		},
		{
			name: "all attempts completed, judge evaluating",
			tsSession: &tripleshot.Session{
				ID:      "ts-1",
				GroupID: "group-1",
				Phase:   tripleshot.PhaseEvaluating,
				Attempts: [3]tripleshot.Attempt{
					{InstanceID: "inst-1", Status: tripleshot.AttemptStatusCompleted},
					{InstanceID: "inst-2", Status: tripleshot.AttemptStatusCompleted},
					{InstanceID: "inst-3", Status: tripleshot.AttemptStatusCompleted},
				},
				JudgeID: "judge-1",
			},
			instances: []*orchestrator.Instance{
				{ID: "inst-1", Status: orchestrator.StatusWorking},
				{ID: "inst-2", Status: orchestrator.StatusWorking},
				{ID: "inst-3", Status: orchestrator.StatusWorking},
				{ID: "judge-1", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 3, // 3 attempts done, judge not done yet
			wantTotal:     4, // 3 attempts + 1 judge
		},
		{
			name: "all complete with evaluation",
			tsSession: &tripleshot.Session{
				ID:      "ts-1",
				GroupID: "group-1",
				Phase:   tripleshot.PhaseComplete,
				Attempts: [3]tripleshot.Attempt{
					{InstanceID: "inst-1", Status: tripleshot.AttemptStatusCompleted},
					{InstanceID: "inst-2", Status: tripleshot.AttemptStatusCompleted},
					{InstanceID: "inst-3", Status: tripleshot.AttemptStatusCompleted},
				},
				JudgeID:    "judge-1",
				Evaluation: &tripleshot.Evaluation{WinnerIndex: 1},
			},
			instances: []*orchestrator.Instance{
				{ID: "inst-1", Status: orchestrator.StatusWorking},
				{ID: "inst-2", Status: orchestrator.StatusWorking},
				{ID: "inst-3", Status: orchestrator.StatusWorking},
				{ID: "judge-1", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 4, // 3 attempts + 1 judge
			wantTotal:     4,
		},
		{
			name: "some attempts failed",
			tsSession: &tripleshot.Session{
				ID:      "ts-1",
				GroupID: "group-1",
				Phase:   tripleshot.PhaseWorking,
				Attempts: [3]tripleshot.Attempt{
					{InstanceID: "inst-1", Status: tripleshot.AttemptStatusCompleted},
					{InstanceID: "inst-2", Status: tripleshot.AttemptStatusFailed},
					{InstanceID: "inst-3", Status: tripleshot.AttemptStatusWorking},
				},
			},
			instances: []*orchestrator.Instance{
				{ID: "inst-1", Status: orchestrator.StatusWorking},
				{ID: "inst-2", Status: orchestrator.StatusError},
				{ID: "inst-3", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 2, // AttemptStatusFailed counts as completed
			wantTotal:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &orchestrator.Session{
				Instances:   tt.instances,
				TripleShots: []*tripleshot.Session{tt.tsSession},
			}

			group := &orchestrator.InstanceGroup{
				ID:          tt.groupID,
				SessionType: string(orchestrator.SessionTypeTripleShot),
				Instances:   make([]string, 0),
			}

			// Add instances to group
			for _, inst := range tt.instances {
				group.Instances = append(group.Instances, inst.ID)
			}

			progress := CalculateGroupProgress(group, session)
			if progress.Completed != tt.wantCompleted {
				t.Errorf("Completed = %d, want %d", progress.Completed, tt.wantCompleted)
			}
			if progress.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", progress.Total, tt.wantTotal)
			}
		})
	}
}

func TestCalculateTripleShotProgress_NoMatchingSession(t *testing.T) {
	// When there's no matching tripleshot session, should fall back to standard counting
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Status: orchestrator.StatusCompleted},
			{ID: "inst-2", Status: orchestrator.StatusWorking},
		},
		TripleShots: []*tripleshot.Session{
			{ID: "ts-1", GroupID: "other-group"}, // Different group
		},
	}

	group := &orchestrator.InstanceGroup{
		ID:          "group-1",
		SessionType: string(orchestrator.SessionTypeTripleShot),
		Instances:   []string{"inst-1", "inst-2"},
	}

	progress := CalculateGroupProgress(group, session)
	if progress.Completed != 1 {
		t.Errorf("Completed = %d, want 1 (fallback to standard counting)", progress.Completed)
	}
	if progress.Total != 2 {
		t.Errorf("Total = %d, want 2 (fallback to standard counting)", progress.Total)
	}
}

func TestCalculateAdversarialProgress(t *testing.T) {
	tests := []struct {
		name          string
		asSession     *adversarial.Session
		instances     []*orchestrator.Instance
		groupID       string
		wantCompleted int
		wantTotal     int
	}{
		{
			name: "just started - implementing",
			asSession: &adversarial.Session{
				ID:           "as-1",
				GroupID:      "group-1",
				Phase:        adversarial.PhaseImplementing,
				CurrentRound: 1,
				History:      []adversarial.Round{},
			},
			instances: []*orchestrator.Instance{
				{ID: "impl-1", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 0,
			wantTotal:     1,
		},
		{
			name: "first round - increment ready, awaiting review",
			asSession: &adversarial.Session{
				ID:           "as-1",
				GroupID:      "group-1",
				Phase:        adversarial.PhaseReviewing,
				CurrentRound: 1,
				History: []adversarial.Round{
					{
						Round:     1,
						Increment: &adversarial.IncrementFile{Round: 1, Status: "ready_for_review"},
					},
				},
			},
			instances: []*orchestrator.Instance{
				{ID: "impl-1", Status: orchestrator.StatusWorking},
				{ID: "rev-1", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 1, // Implementer done
			wantTotal:     2,
		},
		{
			name: "session approved",
			asSession: &adversarial.Session{
				ID:           "as-1",
				GroupID:      "group-1",
				Phase:        adversarial.PhaseApproved,
				CurrentRound: 2,
				History: []adversarial.Round{
					{Round: 1, Increment: &adversarial.IncrementFile{}, Review: &adversarial.ReviewFile{Approved: false}},
					{Round: 2, Increment: &adversarial.IncrementFile{}, Review: &adversarial.ReviewFile{Approved: true}},
				},
			},
			instances: []*orchestrator.Instance{
				{ID: "impl-1", Status: orchestrator.StatusWorking},
				{ID: "rev-1", Status: orchestrator.StatusWorking},
				{ID: "impl-2", Status: orchestrator.StatusWorking},
				{ID: "rev-2", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 4, // All done
			wantTotal:     4,
		},
		{
			name: "session complete",
			asSession: &adversarial.Session{
				ID:           "as-1",
				GroupID:      "group-1",
				Phase:        adversarial.PhaseComplete,
				CurrentRound: 1,
				History: []adversarial.Round{
					{Round: 1, Increment: &adversarial.IncrementFile{}, Review: &adversarial.ReviewFile{Approved: true}},
				},
			},
			instances: []*orchestrator.Instance{
				{ID: "impl-1", Status: orchestrator.StatusWorking},
				{ID: "rev-1", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 2,
			wantTotal:     2,
		},
		{
			name: "session failed",
			asSession: &adversarial.Session{
				ID:           "as-1",
				GroupID:      "group-1",
				Phase:        adversarial.PhaseFailed,
				CurrentRound: 1,
			},
			instances: []*orchestrator.Instance{
				{ID: "impl-1", Status: orchestrator.StatusError},
				{ID: "rev-1", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 1, // Only the errored instance is counted as complete
			wantTotal:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &orchestrator.Session{
				Instances:           tt.instances,
				AdversarialSessions: []*adversarial.Session{tt.asSession},
			}

			group := &orchestrator.InstanceGroup{
				ID:          tt.groupID,
				SessionType: string(orchestrator.SessionTypeAdversarial),
				Instances:   make([]string, 0),
			}

			for _, inst := range tt.instances {
				group.Instances = append(group.Instances, inst.ID)
			}

			progress := CalculateGroupProgress(group, session)
			if progress.Completed != tt.wantCompleted {
				t.Errorf("Completed = %d, want %d", progress.Completed, tt.wantCompleted)
			}
			if progress.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", progress.Total, tt.wantTotal)
			}
		})
	}
}

func TestCalculateRalphProgress(t *testing.T) {
	tests := []struct {
		name          string
		rsSession     *ralph.Session
		instances     []*orchestrator.Instance
		groupID       string
		wantCompleted int
		wantTotal     int
	}{
		{
			name: "first iteration working",
			rsSession: &ralph.Session{
				GroupID:          "group-1",
				Phase:            ralph.PhaseWorking,
				CurrentIteration: 1,
				InstanceID:       "inst-1",
				InstanceIDs:      []string{"inst-1"},
			},
			instances: []*orchestrator.Instance{
				{ID: "inst-1", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 0, // Current instance still working
			wantTotal:     1,
		},
		{
			name: "second iteration working",
			rsSession: &ralph.Session{
				GroupID:          "group-1",
				Phase:            ralph.PhaseWorking,
				CurrentIteration: 2,
				InstanceID:       "inst-2",
				InstanceIDs:      []string{"inst-1", "inst-2"},
			},
			instances: []*orchestrator.Instance{
				{ID: "inst-1", Status: orchestrator.StatusWorking},
				{ID: "inst-2", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 1, // First iteration done, second working
			wantTotal:     2,
		},
		{
			name: "completed successfully",
			rsSession: &ralph.Session{
				GroupID:          "group-1",
				Phase:            ralph.PhaseComplete,
				CurrentIteration: 3,
				InstanceIDs:      []string{"inst-1", "inst-2", "inst-3"},
			},
			instances: []*orchestrator.Instance{
				{ID: "inst-1", Status: orchestrator.StatusWorking},
				{ID: "inst-2", Status: orchestrator.StatusWorking},
				{ID: "inst-3", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 3, // All done
			wantTotal:     3,
		},
		{
			name: "max iterations reached",
			rsSession: &ralph.Session{
				GroupID:          "group-1",
				Phase:            ralph.PhaseMaxIterations,
				CurrentIteration: 5,
				InstanceIDs:      []string{"inst-1", "inst-2", "inst-3", "inst-4", "inst-5"},
			},
			instances: []*orchestrator.Instance{
				{ID: "inst-1", Status: orchestrator.StatusWorking},
				{ID: "inst-2", Status: orchestrator.StatusWorking},
				{ID: "inst-3", Status: orchestrator.StatusWorking},
				{ID: "inst-4", Status: orchestrator.StatusWorking},
				{ID: "inst-5", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 5,
			wantTotal:     5,
		},
		{
			name: "cancelled",
			rsSession: &ralph.Session{
				GroupID:          "group-1",
				Phase:            ralph.PhaseCancelled,
				CurrentIteration: 2,
				InstanceIDs:      []string{"inst-1", "inst-2"},
			},
			instances: []*orchestrator.Instance{
				{ID: "inst-1", Status: orchestrator.StatusWorking},
				{ID: "inst-2", Status: orchestrator.StatusWorking},
			},
			groupID:       "group-1",
			wantCompleted: 2, // All done (cancelled counts as terminal)
			wantTotal:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			tt.rsSession.StartedAt = now

			session := &orchestrator.Session{
				Instances:     tt.instances,
				RalphSessions: []*ralph.Session{tt.rsSession},
			}

			group := &orchestrator.InstanceGroup{
				ID:          tt.groupID,
				SessionType: string(orchestrator.SessionTypeRalph),
				Instances:   make([]string, 0),
			}

			for _, inst := range tt.instances {
				group.Instances = append(group.Instances, inst.ID)
			}

			progress := CalculateGroupProgress(group, session)
			if progress.Completed != tt.wantCompleted {
				t.Errorf("Completed = %d, want %d", progress.Completed, tt.wantCompleted)
			}
			if progress.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", progress.Total, tt.wantTotal)
			}
		})
	}
}

func TestCalculateGroupProgress_TerminalStates(t *testing.T) {
	// Test that terminal states (error, stuck, timeout, interrupted) are counted as completed
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Status: orchestrator.StatusCompleted},
			{ID: "inst-2", Status: orchestrator.StatusError},
			{ID: "inst-3", Status: orchestrator.StatusStuck},
			{ID: "inst-4", Status: orchestrator.StatusTimeout},
			{ID: "inst-5", Status: orchestrator.StatusInterrupted},
			{ID: "inst-6", Status: orchestrator.StatusWorking},
		},
	}

	group := &orchestrator.InstanceGroup{
		ID:        "group-1",
		Instances: []string{"inst-1", "inst-2", "inst-3", "inst-4", "inst-5", "inst-6"},
	}

	progress := CalculateGroupProgress(group, session)
	if progress.Completed != 5 {
		t.Errorf("Completed = %d, want 5 (all terminal states)", progress.Completed)
	}
	if progress.Total != 6 {
		t.Errorf("Total = %d, want 6", progress.Total)
	}
}

func TestCalculateGroupProgress_NilInputs(t *testing.T) {
	t.Run("nil group", func(t *testing.T) {
		session := &orchestrator.Session{}
		progress := CalculateGroupProgress(nil, session)
		if progress.Completed != 0 || progress.Total != 0 {
			t.Errorf("Expected zero progress for nil group, got Completed=%d, Total=%d",
				progress.Completed, progress.Total)
		}
	})

	t.Run("nil session", func(t *testing.T) {
		group := &orchestrator.InstanceGroup{ID: "group-1"}
		progress := CalculateGroupProgress(group, nil)
		if progress.Completed != 0 || progress.Total != 0 {
			t.Errorf("Expected zero progress for nil session, got Completed=%d, Total=%d",
				progress.Completed, progress.Total)
		}
	})

	t.Run("both nil", func(t *testing.T) {
		progress := CalculateGroupProgress(nil, nil)
		if progress.Completed != 0 || progress.Total != 0 {
			t.Errorf("Expected zero progress for nil inputs, got Completed=%d, Total=%d",
				progress.Completed, progress.Total)
		}
	})
}

func TestCalculateTripleShotProgress_WithAdversarialReviewers(t *testing.T) {
	// Test tripleshot with adversarial reviewers enabled
	// Each attempt has a reviewer, plus the judge
	tsSession := &tripleshot.Session{
		ID:      "ts-1",
		GroupID: "group-1",
		Phase:   tripleshot.PhaseAdversarialReview,
		Attempts: [3]tripleshot.Attempt{
			{InstanceID: "impl-1", Status: tripleshot.AttemptStatusCompleted, ReviewerID: "rev-1"},
			{InstanceID: "impl-2", Status: tripleshot.AttemptStatusUnderReview, ReviewerID: "rev-2"},
			{InstanceID: "impl-3", Status: tripleshot.AttemptStatusWorking, ReviewerID: ""},
		},
	}

	instances := []*orchestrator.Instance{
		{ID: "impl-1", Status: orchestrator.StatusWorking},
		{ID: "impl-2", Status: orchestrator.StatusWorking},
		{ID: "impl-3", Status: orchestrator.StatusWorking},
		{ID: "rev-1", Status: orchestrator.StatusCompleted},
		{ID: "rev-2", Status: orchestrator.StatusWorking},
	}

	session := &orchestrator.Session{
		Instances:   instances,
		TripleShots: []*tripleshot.Session{tsSession},
	}

	group := &orchestrator.InstanceGroup{
		ID:          "group-1",
		SessionType: string(orchestrator.SessionTypeTripleShot),
		Instances:   []string{"impl-1", "impl-2", "impl-3", "rev-1", "rev-2"},
	}

	progress := CalculateGroupProgress(group, session)
	// Expected: impl-1 completed (AttemptStatusCompleted), rev-1 completed (StatusCompleted)
	// impl-2 not completed (UnderReview), rev-2 not completed (Working)
	// impl-3 not completed (Working)
	// Total = 3 attempts + 2 reviewers = 5 (no judge yet)
	if progress.Total != 5 {
		t.Errorf("Total = %d, want 5", progress.Total)
	}
	// Completed: impl-1 (attempt completed) + rev-1 (instance completed) = 2
	if progress.Completed != 2 {
		t.Errorf("Completed = %d, want 2", progress.Completed)
	}
}

func TestFindWorkflowSessions(t *testing.T) {
	t.Run("findTripleShotSession", func(t *testing.T) {
		session := &orchestrator.Session{
			TripleShots: []*tripleshot.Session{
				{ID: "ts-1", GroupID: "group-1"},
				{ID: "ts-2", GroupID: "group-2"},
			},
		}

		found := findTripleShotSession("group-1", session)
		if found == nil || found.ID != "ts-1" {
			t.Errorf("findTripleShotSession(group-1) = %v, want ts-1", found)
		}

		notFound := findTripleShotSession("group-99", session)
		if notFound != nil {
			t.Errorf("findTripleShotSession(group-99) = %v, want nil", notFound)
		}
	})

	t.Run("findAdversarialSession", func(t *testing.T) {
		session := &orchestrator.Session{
			AdversarialSessions: []*adversarial.Session{
				{ID: "as-1", GroupID: "group-1"},
				{ID: "as-2", GroupID: "group-2"},
			},
		}

		found := findAdversarialSession("group-1", session)
		if found == nil || found.ID != "as-1" {
			t.Errorf("findAdversarialSession(group-1) = %v, want as-1", found)
		}

		notFound := findAdversarialSession("group-99", session)
		if notFound != nil {
			t.Errorf("findAdversarialSession(group-99) = %v, want nil", notFound)
		}
	})

	t.Run("findRalphSession", func(t *testing.T) {
		now := time.Now()
		session := &orchestrator.Session{
			RalphSessions: []*ralph.Session{
				{GroupID: "group-1", StartedAt: now},
				{GroupID: "group-2", StartedAt: now},
			},
		}

		found := findRalphSession("group-1", session)
		if found == nil || found.GroupID != "group-1" {
			t.Errorf("findRalphSession(group-1) = %v, want group-1", found)
		}

		notFound := findRalphSession("group-99", session)
		if notFound != nil {
			t.Errorf("findRalphSession(group-99) = %v, want nil", notFound)
		}
	})
}

func TestCalculateAdversarialProgress_MultipleRoundsInProgress(t *testing.T) {
	tests := []struct {
		name          string
		asSession     *adversarial.Session
		instances     []*orchestrator.Instance
		wantCompleted int
		wantTotal     int
	}{
		{
			name: "round 1 rejected, round 2 implementing",
			asSession: &adversarial.Session{
				ID:           "as-1",
				GroupID:      "group-1",
				Phase:        adversarial.PhaseImplementing,
				CurrentRound: 2,
				History: []adversarial.Round{
					{Round: 1, Increment: &adversarial.IncrementFile{}, Review: &adversarial.ReviewFile{Approved: false}},
					{Round: 2}, // Round 2 just started, no increment yet
				},
			},
			instances: []*orchestrator.Instance{
				{ID: "impl-1", Status: orchestrator.StatusCompleted},
				{ID: "rev-1", Status: orchestrator.StatusCompleted},
				{ID: "impl-2", Status: orchestrator.StatusWorking},
			},
			wantCompleted: 2, // Round 1 complete (impl-1 + rev-1)
			wantTotal:     3,
		},
		{
			name: "round 1 rejected, round 2 increment ready",
			asSession: &adversarial.Session{
				ID:           "as-1",
				GroupID:      "group-1",
				Phase:        adversarial.PhaseReviewing,
				CurrentRound: 2,
				History: []adversarial.Round{
					{Round: 1, Increment: &adversarial.IncrementFile{}, Review: &adversarial.ReviewFile{Approved: false}},
					{Round: 2, Increment: &adversarial.IncrementFile{}}, // Increment ready, no review yet
				},
			},
			instances: []*orchestrator.Instance{
				{ID: "impl-1", Status: orchestrator.StatusCompleted},
				{ID: "rev-1", Status: orchestrator.StatusCompleted},
				{ID: "impl-2", Status: orchestrator.StatusCompleted},
				{ID: "rev-2", Status: orchestrator.StatusWorking},
			},
			wantCompleted: 3, // Round 1 (2) + Round 2 implementer (1)
			wantTotal:     4,
		},
		{
			name: "multiple rounds rejected, round 3 in progress",
			asSession: &adversarial.Session{
				ID:           "as-1",
				GroupID:      "group-1",
				Phase:        adversarial.PhaseReviewing,
				CurrentRound: 3,
				History: []adversarial.Round{
					{Round: 1, Increment: &adversarial.IncrementFile{}, Review: &adversarial.ReviewFile{Approved: false}},
					{Round: 2, Increment: &adversarial.IncrementFile{}, Review: &adversarial.ReviewFile{Approved: false}},
					{Round: 3, Increment: &adversarial.IncrementFile{}}, // Increment ready
				},
			},
			instances: []*orchestrator.Instance{
				{ID: "impl-1", Status: orchestrator.StatusCompleted},
				{ID: "rev-1", Status: orchestrator.StatusCompleted},
				{ID: "impl-2", Status: orchestrator.StatusCompleted},
				{ID: "rev-2", Status: orchestrator.StatusCompleted},
				{ID: "impl-3", Status: orchestrator.StatusCompleted},
				{ID: "rev-3", Status: orchestrator.StatusWorking},
			},
			wantCompleted: 5, // Round 1 (2) + Round 2 (2) + Round 3 implementer (1)
			wantTotal:     6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &orchestrator.Session{
				Instances:           tt.instances,
				AdversarialSessions: []*adversarial.Session{tt.asSession},
			}

			group := &orchestrator.InstanceGroup{
				ID:          "group-1",
				SessionType: string(orchestrator.SessionTypeAdversarial),
				Instances:   make([]string, 0),
			}

			for _, inst := range tt.instances {
				group.Instances = append(group.Instances, inst.ID)
			}

			progress := CalculateGroupProgress(group, session)
			if progress.Completed != tt.wantCompleted {
				t.Errorf("Completed = %d, want %d", progress.Completed, tt.wantCompleted)
			}
			if progress.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", progress.Total, tt.wantTotal)
			}
		})
	}
}
