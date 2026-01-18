package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Coverage note: Full ProcessReviewCompletion tests require filesystem setup
// for the review file. These tests verify the score enforcement logic.

func TestAdversarialCoordinator_ScoreEnforcementBeforeCallbacks(t *testing.T) {
	// This test verifies that when a reviewer approves with a score below the
	// minimum threshold, the OnRejected callback is called (not OnApproved).
	// This is the bug fix for users who set higher minimum score thresholds.

	tests := []struct {
		name             string
		reviewApproved   bool
		reviewScore      int
		minPassingScore  int
		expectApproved   bool
		expectRejected   bool
		expectApprovalIn *bool // The approval state that should be passed to callbacks
	}{
		{
			name:             "approved with score meeting threshold",
			reviewApproved:   true,
			reviewScore:      9,
			minPassingScore:  9,
			expectApproved:   true,
			expectRejected:   false,
			expectApprovalIn: boolPtr(true),
		},
		{
			name:             "approved with score exceeding threshold",
			reviewApproved:   true,
			reviewScore:      10,
			minPassingScore:  8,
			expectApproved:   true,
			expectRejected:   false,
			expectApprovalIn: boolPtr(true),
		},
		{
			name:             "approved but score below threshold - should be enforced to rejected",
			reviewApproved:   true,
			reviewScore:      8,
			minPassingScore:  9,
			expectApproved:   false,
			expectRejected:   true,
			expectApprovalIn: boolPtr(false),
		},
		{
			name:             "approved with score 7 but threshold 8 - should be enforced",
			reviewApproved:   true,
			reviewScore:      7,
			minPassingScore:  8,
			expectApproved:   false,
			expectRejected:   true,
			expectApprovalIn: boolPtr(false),
		},
		{
			name:             "rejected stays rejected",
			reviewApproved:   false,
			reviewScore:      5,
			minPassingScore:  8,
			expectApproved:   false,
			expectRejected:   true,
			expectApprovalIn: boolPtr(false),
		},
		{
			name:             "high threshold of 10 requires perfect score",
			reviewApproved:   true,
			reviewScore:      9,
			minPassingScore:  10,
			expectApproved:   false,
			expectRejected:   true,
			expectApprovalIn: boolPtr(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for review file
			tmpDir := t.TempDir()

			// Create adversarial session with min passing score
			// For rejection cases, set MaxIterations=1 and CurrentRound=2 so
			// IsMaxIterationsReached() returns true and we don't try to start a new implementer
			maxIterations := 10
			currentRound := 1
			if !tt.expectApproved {
				// When we expect rejection, ensure max iterations is reached
				// so the code doesn't try to start a new implementer
				maxIterations = 1
				currentRound = 2
			}

			// Create review file with matching round
			review := AdversarialReviewFile{
				Round:    currentRound,
				Approved: tt.reviewApproved,
				Score:    tt.reviewScore,
				Summary:  "Test review",
			}
			reviewData, err := json.Marshal(review)
			if err != nil {
				t.Fatalf("failed to marshal review: %v", err)
			}
			reviewPath := filepath.Join(tmpDir, AdversarialReviewFileName)
			if err := os.WriteFile(reviewPath, reviewData, 0644); err != nil {
				t.Fatalf("failed to write review file: %v", err)
			}

			config := AdversarialConfig{
				MaxIterations:   maxIterations,
				MinPassingScore: tt.minPassingScore,
			}
			advSession := NewAdversarialSession("Test task", config)
			advSession.CurrentRound = currentRound
			advSession.History = []AdversarialRound{{Round: currentRound}}

			// Create coordinator with callbacks to track what state they receive
			var approvedCalled bool
			var rejectedCalled bool
			var reviewReadyApprovalState *bool

			coord := NewAdversarialCoordinator(nil, nil, advSession, nil)
			coord.reviewerWorktree = tmpDir
			coord.SetCallbacks(&AdversarialCoordinatorCallbacks{
				OnReviewReady: func(round int, r *AdversarialReviewFile) {
					// Capture the approval state passed to callback
					reviewReadyApprovalState = boolPtr(r.Approved)
				},
				OnApproved: func(round int, r *AdversarialReviewFile) {
					approvedCalled = true
				},
				OnRejected: func(round int, r *AdversarialReviewFile) {
					rejectedCalled = true
				},
				OnComplete: func(success bool, summary string) {
					// No-op for this test
				},
				OnPhaseChange: func(phase AdversarialPhase) {
					// No-op for this test
				},
			})

			// Process the review
			_ = coord.ProcessReviewCompletion()

			// Verify callbacks were called correctly
			if approvedCalled != tt.expectApproved {
				t.Errorf("OnApproved called = %v, want %v", approvedCalled, tt.expectApproved)
			}
			if rejectedCalled != tt.expectRejected {
				t.Errorf("OnRejected called = %v, want %v", rejectedCalled, tt.expectRejected)
			}

			// Verify the approval state passed to OnReviewReady callback
			if reviewReadyApprovalState == nil {
				t.Error("OnReviewReady was not called")
			} else if *reviewReadyApprovalState != *tt.expectApprovalIn {
				t.Errorf("OnReviewReady received approval = %v, want %v", *reviewReadyApprovalState, *tt.expectApprovalIn)
			}
		})
	}
}

func TestAdversarialCoordinator_RequiredChangesAddedOnEnforcement(t *testing.T) {
	// When the score enforcement overrides approval, it should add a required change
	// explaining why (if no required changes exist).

	tmpDir := t.TempDir()

	// Create review file with approval=true, score=7 (below default 8)
	// Use round 2 to match the session's current round
	review := AdversarialReviewFile{
		Round:           2,
		Approved:        true,
		Score:           7,
		Summary:         "Looks good",
		RequiredChanges: []string{}, // Empty - enforcement should add one
	}
	reviewData, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("failed to marshal review: %v", err)
	}
	reviewPath := filepath.Join(tmpDir, AdversarialReviewFileName)
	if err := os.WriteFile(reviewPath, reviewData, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	// Use MaxIterations=1, CurrentRound=2 to trigger max iterations path
	config := AdversarialConfig{
		MaxIterations:   1,
		MinPassingScore: 8,
	}
	advSession := NewAdversarialSession("Test task", config)
	advSession.CurrentRound = 2
	advSession.History = []AdversarialRound{{Round: 2}}

	var capturedReview *AdversarialReviewFile
	coord := NewAdversarialCoordinator(nil, nil, advSession, nil)
	coord.reviewerWorktree = tmpDir
	coord.SetCallbacks(&AdversarialCoordinatorCallbacks{
		OnReviewReady: func(round int, r *AdversarialReviewFile) {
			capturedReview = r
		},
		OnRejected:    func(round int, r *AdversarialReviewFile) {},
		OnComplete:    func(success bool, summary string) {},
		OnPhaseChange: func(phase AdversarialPhase) {},
	})

	_ = coord.ProcessReviewCompletion()

	if capturedReview == nil {
		t.Fatal("expected review to be captured")
	}

	if len(capturedReview.RequiredChanges) == 0 {
		t.Error("expected required changes to be added when enforcement overrides approval")
	}

	// Check the message mentions the score threshold
	found := false
	for _, change := range capturedReview.RequiredChanges {
		if strings.Contains(change, "7") && strings.Contains(change, "8") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected required change to mention score (7) and threshold (8), got: %v", capturedReview.RequiredChanges)
	}
}

func TestAdversarialCoordinator_ExistingRequiredChangesPreserved(t *testing.T) {
	// When enforcement happens but there are already required changes,
	// don't overwrite them.

	tmpDir := t.TempDir()

	// Use round 2 to match the session's current round
	review := AdversarialReviewFile{
		Round:           2,
		Approved:        true,
		Score:           7,
		Summary:         "Needs work",
		RequiredChanges: []string{"Fix the bug", "Add tests"},
	}
	reviewData, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("failed to marshal review: %v", err)
	}
	reviewPath := filepath.Join(tmpDir, AdversarialReviewFileName)
	if err := os.WriteFile(reviewPath, reviewData, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	// Use MaxIterations=1, CurrentRound=2 to trigger max iterations path
	config := AdversarialConfig{
		MaxIterations:   1,
		MinPassingScore: 8,
	}
	advSession := NewAdversarialSession("Test task", config)
	advSession.CurrentRound = 2
	advSession.History = []AdversarialRound{{Round: 2}}

	var capturedReview *AdversarialReviewFile
	coord := NewAdversarialCoordinator(nil, nil, advSession, nil)
	coord.reviewerWorktree = tmpDir
	coord.SetCallbacks(&AdversarialCoordinatorCallbacks{
		OnReviewReady: func(round int, r *AdversarialReviewFile) {
			capturedReview = r
		},
		OnRejected:    func(round int, r *AdversarialReviewFile) {},
		OnComplete:    func(success bool, summary string) {},
		OnPhaseChange: func(phase AdversarialPhase) {},
	})

	_ = coord.ProcessReviewCompletion()

	if capturedReview == nil {
		t.Fatal("expected review to be captured")
	}

	// Should still have the original required changes
	if len(capturedReview.RequiredChanges) != 2 {
		t.Errorf("expected original 2 required changes, got %d", len(capturedReview.RequiredChanges))
	}
}

func TestAdversarialCoordinator_InvalidMinPassingScoreFallback(t *testing.T) {
	// When MinPassingScore is invalid (0 or >10), should fallback to 8

	tests := []struct {
		name            string
		minPassingScore int
		reviewScore     int
		expectApproved  bool
	}{
		{
			name:            "zero min score uses fallback 8, score 7 rejected",
			minPassingScore: 0,
			reviewScore:     7,
			expectApproved:  false,
		},
		{
			name:            "zero min score uses fallback 8, score 8 approved",
			minPassingScore: 0,
			reviewScore:     8,
			expectApproved:  true,
		},
		{
			name:            "negative min score uses fallback 8",
			minPassingScore: -1,
			reviewScore:     7,
			expectApproved:  false,
		},
		{
			name:            "min score 11 uses fallback 8",
			minPassingScore: 11,
			reviewScore:     8,
			expectApproved:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// For rejection cases, use MaxIterations=1, CurrentRound=2 to trigger max iterations path
			maxIterations := 10
			currentRound := 1
			if !tt.expectApproved {
				maxIterations = 1
				currentRound = 2
			}

			review := AdversarialReviewFile{
				Round:    currentRound,
				Approved: true,
				Score:    tt.reviewScore,
				Summary:  "Test",
			}
			reviewData, err := json.Marshal(review)
			if err != nil {
				t.Fatalf("failed to marshal review: %v", err)
			}
			if err := os.WriteFile(filepath.Join(tmpDir, AdversarialReviewFileName), reviewData, 0644); err != nil {
				t.Fatalf("failed to write review file: %v", err)
			}

			config := AdversarialConfig{
				MaxIterations:   maxIterations,
				MinPassingScore: tt.minPassingScore,
			}
			advSession := NewAdversarialSession("Test", config)
			advSession.CurrentRound = currentRound
			advSession.History = []AdversarialRound{{Round: currentRound}}

			var approvedCalled bool
			coord := NewAdversarialCoordinator(nil, nil, advSession, nil)
			coord.reviewerWorktree = tmpDir
			coord.SetCallbacks(&AdversarialCoordinatorCallbacks{
				OnReviewReady: func(round int, r *AdversarialReviewFile) {},
				OnApproved: func(round int, r *AdversarialReviewFile) {
					approvedCalled = true
				},
				OnRejected:    func(round int, r *AdversarialReviewFile) {},
				OnComplete:    func(success bool, summary string) {},
				OnPhaseChange: func(phase AdversarialPhase) {},
			})

			_ = coord.ProcessReviewCompletion()

			if approvedCalled != tt.expectApproved {
				t.Errorf("OnApproved called = %v, want %v (score=%d, configured min=%d, effective min=8)",
					approvedCalled, tt.expectApproved, tt.reviewScore, tt.minPassingScore)
			}
		})
	}
}

// Helper functions

func boolPtr(b bool) *bool {
	return &b
}
