package adversarial

import (
	"strings"
	"testing"
)

func TestImplementerPromptTemplate(t *testing.T) {
	// Verify the template contains expected placeholders
	if len(ImplementerPromptTemplate) == 0 {
		t.Error("ImplementerPromptTemplate should not be empty")
	}
	if !strings.Contains(ImplementerPromptTemplate, "%s") {
		t.Error("template should contain task placeholder")
	}
	if !strings.Contains(ImplementerPromptTemplate, "%d") {
		t.Error("template should contain round placeholder")
	}
	if !strings.Contains(ImplementerPromptTemplate, IncrementFileName) {
		t.Error("template should reference increment file name")
	}
}

func TestReviewerPromptTemplate(t *testing.T) {
	if len(ReviewerPromptTemplate) == 0 {
		t.Error("ReviewerPromptTemplate should not be empty")
	}
	if !strings.Contains(ReviewerPromptTemplate, "%s") {
		t.Error("template should contain task placeholder")
	}
	if !strings.Contains(ReviewerPromptTemplate, "%d") {
		t.Error("template should contain round placeholder")
	}
	if !strings.Contains(ReviewerPromptTemplate, ReviewFileName) {
		t.Error("template should reference review file name")
	}
}

func TestFormatImplementerPrompt_FirstRound(t *testing.T) {
	task := "Implement a rate limiter"
	round := 1

	prompt := FormatImplementerPrompt(task, round, nil, "")

	if !strings.Contains(prompt, task) {
		t.Error("prompt should contain the task")
	}
	if !strings.Contains(prompt, "Current Round: 1") {
		t.Error("prompt should indicate round 1")
	}
	if strings.Contains(prompt, "Previous Review Feedback") {
		t.Error("first round should not have previous feedback")
	}
}

func TestFormatImplementerPrompt_WithPreviousFeedback(t *testing.T) {
	task := "Implement a rate limiter"
	round := 2
	previousReview := &ReviewFile{
		Round:    1,
		Approved: false,
		Score:    6,
		Issues:   []string{"Missing error handling", "No tests"},
		RequiredChanges: []string{
			"Add error handling for nil inputs",
			"Add unit tests",
		},
		Summary: "Good start but needs improvement",
	}

	prompt := FormatImplementerPrompt(task, round, previousReview, "")

	if !strings.Contains(prompt, task) {
		t.Error("prompt should contain the task")
	}
	if !strings.Contains(prompt, "Current Round: 2") {
		t.Error("prompt should indicate round 2")
	}
	if !strings.Contains(prompt, "Previous Review Feedback") {
		t.Error("prompt should contain previous feedback section")
	}
	if !strings.Contains(prompt, "**Score:** 6/10") {
		t.Error("prompt should show previous score")
	}
	if !strings.Contains(prompt, "Missing error handling") {
		t.Error("prompt should contain previous issues")
	}
	if !strings.Contains(prompt, "Add error handling for nil inputs") {
		t.Error("prompt should contain required changes")
	}
	if !strings.Contains(prompt, "Good start but needs improvement") {
		t.Error("prompt should contain reviewer's summary")
	}
}

func TestFormatImplementerPrompt_EmptyIssuesAndChanges(t *testing.T) {
	task := "Test task"
	round := 2
	previousReview := &ReviewFile{
		Round:           1,
		Approved:        false,
		Score:           5,
		Issues:          []string{},
		RequiredChanges: []string{},
		Summary:         "Needs work",
	}

	prompt := FormatImplementerPrompt(task, round, previousReview, "")

	// Should include "(none specified)" for empty lists
	if !strings.Contains(prompt, "(none specified)") {
		t.Error("prompt should indicate when no issues/changes are specified")
	}
}

func TestFormatReviewerPrompt(t *testing.T) {
	task := "Implement a rate limiter"
	round := 1
	increment := &IncrementFile{
		Round:         1,
		Status:        "ready_for_review",
		Summary:       "Implemented basic rate limiting",
		FilesModified: []string{"ratelimiter.go", "ratelimiter_test.go"},
		Approach:      "Used token bucket algorithm",
		Notes:         "Consider thread safety",
	}
	minPassingScore := 8

	prompt := FormatReviewerPrompt(task, round, increment, minPassingScore, "")

	if !strings.Contains(prompt, task) {
		t.Error("prompt should contain the original task")
	}
	if !strings.Contains(prompt, "Current Round: 1") {
		t.Error("prompt should indicate round 1")
	}
	if !strings.Contains(prompt, "Implemented basic rate limiting") {
		t.Error("prompt should contain increment summary")
	}
	if !strings.Contains(prompt, "token bucket algorithm") {
		t.Error("prompt should contain approach")
	}
	if !strings.Contains(prompt, "ratelimiter.go") {
		t.Error("prompt should contain modified files")
	}
	if !strings.Contains(prompt, "Consider thread safety") {
		t.Error("prompt should contain implementer notes")
	}
	if !strings.Contains(prompt, "score >= 8") {
		t.Error("prompt should indicate minimum passing score")
	}
}

func TestFormatReviewerPrompt_DifferentMinScores(t *testing.T) {
	task := "Test task"
	round := 1
	increment := &IncrementFile{
		Round:   1,
		Status:  "ready_for_review",
		Summary: "Test",
	}

	tests := []struct {
		minScore int
		expected string
	}{
		{7, "score >= 7"},
		{8, "score >= 8"},
		{9, "score >= 9"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			prompt := FormatReviewerPrompt(task, round, increment, tt.minScore, "")
			if !strings.Contains(prompt, tt.expected) {
				t.Errorf("prompt should contain %q", tt.expected)
			}
		})
	}
}

func TestPreviousFeedbackTemplate(t *testing.T) {
	if len(PreviousFeedbackTemplate) == 0 {
		t.Error("PreviousFeedbackTemplate should not be empty")
	}
	if !strings.Contains(PreviousFeedbackTemplate, "Previous Review Feedback") {
		t.Error("template should mention previous review")
	}
	if !strings.Contains(PreviousFeedbackTemplate, "Issues to Fix") {
		t.Error("template should have issues section")
	}
	if !strings.Contains(PreviousFeedbackTemplate, "Required Changes") {
		t.Error("template should have required changes section")
	}
}

func TestImplementerPromptTemplate_CompletionProtocol(t *testing.T) {
	// Verify emphatic completion protocol wording
	expectedParts := []string{
		"FINAL MANDATORY STEP",
		"FINAL MANDATORY ACTION",
		"BLOCKED waiting",
		"DO NOT",
		"wait for user prompting",
		"Write this file AUTOMATICALLY",
		"REMEMBER",
		"Your implementation is NOT complete until you write this file",
	}

	for _, part := range expectedParts {
		if !strings.Contains(ImplementerPromptTemplate, part) {
			t.Errorf("Completion protocol missing %q", part)
		}
	}
}

func TestReviewerPromptTemplate_CompletionProtocol(t *testing.T) {
	// Verify emphatic completion protocol wording
	expectedParts := []string{
		"FINAL MANDATORY STEP",
		"FINAL MANDATORY ACTION",
		"BLOCKED waiting",
		"DO NOT",
		"wait for user prompting",
		"Write this file AUTOMATICALLY",
		"REMEMBER",
		"Your review is NOT complete until you write this file",
	}

	for _, part := range expectedParts {
		if !strings.Contains(ReviewerPromptTemplate, part) {
			t.Errorf("Completion protocol missing %q", part)
		}
	}
}

func TestFormatImplementerPrompt_WithWorktreePath(t *testing.T) {
	task := "Implement a feature"
	round := 1
	worktreePath := "/path/to/worktree"

	prompt := FormatImplementerPrompt(task, round, nil, worktreePath)

	// Should contain the absolute path to the increment file
	expectedPath := "/path/to/worktree/" + IncrementFileName
	if !strings.Contains(prompt, expectedPath) {
		t.Errorf("prompt should contain absolute path %q", expectedPath)
	}

	// Should have the warning about writing to the exact path
	if !strings.Contains(prompt, "EXACT path") {
		t.Error("prompt should contain warning about writing to exact path")
	}
}

func TestFormatReviewerPrompt_WithWorktreePath(t *testing.T) {
	task := "Review a feature"
	round := 1
	increment := &IncrementFile{
		Round:   1,
		Status:  "ready_for_review",
		Summary: "Test",
	}
	worktreePath := "/path/to/worktree"

	prompt := FormatReviewerPrompt(task, round, increment, 8, worktreePath)

	// Should contain the absolute path to the review file
	expectedPath := "/path/to/worktree/" + ReviewFileName
	if !strings.Contains(prompt, expectedPath) {
		t.Errorf("prompt should contain absolute path %q", expectedPath)
	}

	// Should have the warning about writing to the exact path
	if !strings.Contains(prompt, "EXACT path") {
		t.Error("prompt should contain warning about writing to exact path")
	}
}
