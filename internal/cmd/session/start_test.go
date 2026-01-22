package session

import (
	"errors"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// mockLogger captures log calls for verification in tests.
type mockLogger struct {
	warnCalls []logCall
	infoCalls []logCall
}

type logCall struct {
	msg  string
	args []any
}

func (m *mockLogger) warn(msg string, args ...any) {
	m.warnCalls = append(m.warnCalls, logCall{msg: msg, args: args})
}

func (m *mockLogger) info(msg string, args ...any) {
	m.infoCalls = append(m.infoCalls, logCall{msg: msg, args: args})
}

// TestResumeMultiPassPlanningInternal_NoPlanners tests that when there are no
// existing planner IDs, the function calls RunPlanning to start fresh.
func TestResumeMultiPassPlanningInternal_NoPlanners(t *testing.T) {
	logger := &mockLogger{}
	runPlanningCalled := false

	deps := multiPassResumeDeps{
		getInstance:   func(id string) *orchestrator.Instance { return nil },
		isTmuxRunning: func(id string) bool { return false },
		saveSession:   func() error { return nil },
		runPlanning: func() error {
			runPlanningCalled = true
			return nil
		},
		runPlanManager: func() error { return nil },
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			return nil, errors.New("not called")
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{}, // No planners
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !runPlanningCalled {
		t.Error("expected RunPlanning to be called when no planners exist")
	}
}

// TestResumeMultiPassPlanningInternal_PlannersStillRunning tests that when
// some planners are still running, we don't trigger the evaluator.
func TestResumeMultiPassPlanningInternal_PlannersStillRunning(t *testing.T) {
	logger := &mockLogger{}
	runPlanManagerCalled := false

	// Instances for planners
	instances := map[string]*orchestrator.Instance{
		"p1": {ID: "p1", WorktreePath: "/tmp/p1"},
		"p2": {ID: "p2", WorktreePath: "/tmp/p2"},
		"p3": {ID: "p3", WorktreePath: "/tmp/p3"},
	}

	// p1 still running, p2 and p3 completed
	tmuxRunning := map[string]bool{
		"p1": true,
		"p2": false,
		"p3": false,
	}

	deps := multiPassResumeDeps{
		getInstance: func(id string) *orchestrator.Instance {
			return instances[id]
		},
		isTmuxRunning: func(id string) bool {
			return tmuxRunning[id]
		},
		saveSession: func() error { return nil },
		runPlanning: func() error {
			t.Error("RunPlanning should not be called when planners exist")
			return nil
		},
		runPlanManager: func() error {
			runPlanManagerCalled = true
			return nil
		},
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			return &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "t1"}}}, nil
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{"p1", "p2", "p3"},
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if runPlanManagerCalled {
		t.Error("RunPlanManager should not be called when planners are still running")
	}
}

// TestResumeMultiPassPlanningInternal_AllCompleted_TriggersEvaluator tests
// that when all planners are completed and plans are collected, the evaluator
// is triggered.
func TestResumeMultiPassPlanningInternal_AllCompleted_TriggersEvaluator(t *testing.T) {
	logger := &mockLogger{}
	runPlanManagerCalled := false
	saveSessionCalled := false

	instances := map[string]*orchestrator.Instance{
		"p1": {ID: "p1", WorktreePath: "/tmp/p1"},
		"p2": {ID: "p2", WorktreePath: "/tmp/p2"},
		"p3": {ID: "p3", WorktreePath: "/tmp/p3"},
	}

	deps := multiPassResumeDeps{
		getInstance: func(id string) *orchestrator.Instance {
			return instances[id]
		},
		isTmuxRunning: func(id string) bool {
			return false // All completed
		},
		saveSession: func() error {
			saveSessionCalled = true
			return nil
		},
		runPlanning: func() error {
			t.Error("RunPlanning should not be called")
			return nil
		},
		runPlanManager: func() error {
			runPlanManagerCalled = true
			return nil
		},
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			return &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "t1"}}}, nil
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{"p1", "p2", "p3"},
		Objective:          "test objective",
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !runPlanManagerCalled {
		t.Error("expected RunPlanManager to be called when all planners completed")
	}
	if !saveSessionCalled {
		t.Error("expected SaveSession to be called after triggering evaluator")
	}
	// Verify all planners were marked as processed
	if len(ultraSession.ProcessedCoordinators) != 3 {
		t.Errorf("expected 3 processed coordinators, got %d", len(ultraSession.ProcessedCoordinators))
	}
	// Verify plans were collected
	validPlans := 0
	for _, p := range ultraSession.CandidatePlans {
		if p != nil {
			validPlans++
		}
	}
	if validPlans != 3 {
		t.Errorf("expected 3 valid plans, got %d", validPlans)
	}
}

// TestResumeMultiPassPlanningInternal_NilInstance_MarkedAsCompleted tests
// the edge case where GetInstance returns nil for a planner. The index should
// still be added to completedPlanners and properly processed to avoid false
// negatives in the all-processed check.
func TestResumeMultiPassPlanningInternal_NilInstance_MarkedAsCompleted(t *testing.T) {
	logger := &mockLogger{}
	runPlanManagerCalled := false

	// Only p1 and p2 exist, p3 is nil (was deleted or never existed)
	instances := map[string]*orchestrator.Instance{
		"p1": {ID: "p1", WorktreePath: "/tmp/p1"},
		"p2": {ID: "p2", WorktreePath: "/tmp/p2"},
		// p3 intentionally missing
	}

	deps := multiPassResumeDeps{
		getInstance: func(id string) *orchestrator.Instance {
			return instances[id] // Returns nil for p3
		},
		isTmuxRunning: func(id string) bool {
			return false // All completed
		},
		saveSession: func() error { return nil },
		runPlanning: func() error {
			t.Error("RunPlanning should not be called")
			return nil
		},
		runPlanManager: func() error {
			runPlanManagerCalled = true
			return nil
		},
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			return &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "t1"}}}, nil
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{"p1", "p2", "p3"}, // p3 will return nil
		Objective:          "test objective",
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !runPlanManagerCalled {
		t.Error("expected RunPlanManager to be called even with nil instance")
	}
	// Verify all planners were marked as processed (including the nil one)
	if len(ultraSession.ProcessedCoordinators) != 3 {
		t.Errorf("expected 3 processed coordinators, got %d", len(ultraSession.ProcessedCoordinators))
	}
	// Verify p3 (index 2) was marked as processed
	if !ultraSession.ProcessedCoordinators[2] {
		t.Error("expected index 2 (nil instance) to be marked as processed")
	}

	// Verify warning was logged for nil instance
	foundNilWarning := false
	for _, call := range logger.warnCalls {
		if call.msg == "planner instance not found in session" {
			foundNilWarning = true
			break
		}
	}
	if !foundNilWarning {
		t.Error("expected warning to be logged for nil instance")
	}
}

// TestResumeMultiPassPlanningInternal_EvaluatorAlreadyStarted tests that
// when PlanManagerID is already set, we don't trigger another evaluator.
func TestResumeMultiPassPlanningInternal_EvaluatorAlreadyStarted(t *testing.T) {
	logger := &mockLogger{}
	runPlanManagerCalled := false

	instances := map[string]*orchestrator.Instance{
		"p1": {ID: "p1", WorktreePath: "/tmp/p1"},
		"p2": {ID: "p2", WorktreePath: "/tmp/p2"},
		"p3": {ID: "p3", WorktreePath: "/tmp/p3"},
	}

	deps := multiPassResumeDeps{
		getInstance: func(id string) *orchestrator.Instance {
			return instances[id]
		},
		isTmuxRunning: func(id string) bool {
			return false // All completed
		},
		saveSession: func() error { return nil },
		runPlanning: func() error { return nil },
		runPlanManager: func() error {
			runPlanManagerCalled = true
			return nil
		},
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			return &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "t1"}}}, nil
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{"p1", "p2", "p3"},
		PlanManagerID:      "evaluator-123", // Already set
		ProcessedCoordinators: map[int]bool{
			0: true,
			1: true,
			2: true,
		},
		CandidatePlans: []*orchestrator.PlanSpec{
			{Tasks: []orchestrator.PlannedTask{{ID: "t1"}}},
			{Tasks: []orchestrator.PlannedTask{{ID: "t2"}}},
			{Tasks: []orchestrator.PlannedTask{{ID: "t3"}}},
		},
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if runPlanManagerCalled {
		t.Error("RunPlanManager should not be called when evaluator already started")
	}
}

// TestResumeMultiPassPlanningInternal_NoValidPlans tests that when all
// planners complete but no valid plans are produced, an error is returned.
func TestResumeMultiPassPlanningInternal_NoValidPlans(t *testing.T) {
	logger := &mockLogger{}

	instances := map[string]*orchestrator.Instance{
		"p1": {ID: "p1", WorktreePath: "/tmp/p1"},
		"p2": {ID: "p2", WorktreePath: "/tmp/p2"},
		"p3": {ID: "p3", WorktreePath: "/tmp/p3"},
	}

	deps := multiPassResumeDeps{
		getInstance: func(id string) *orchestrator.Instance {
			return instances[id]
		},
		isTmuxRunning: func(id string) bool {
			return false // All completed
		},
		saveSession: func() error { return nil },
		runPlanning: func() error { return nil },
		runPlanManager: func() error {
			t.Error("RunPlanManager should not be called when no valid plans")
			return nil
		},
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			return nil, errors.New("plan parse failed") // All fail
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{"p1", "p2", "p3"},
		Objective:          "test objective",
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err == nil {
		t.Error("expected error when no valid plans produced")
	}
	if err.Error() != "all multi-pass planners completed but no valid plans were produced" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestResumeMultiPassPlanningInternal_PartialPlanFailure tests that when
// some plans fail to parse but others succeed, the evaluator is still triggered.
func TestResumeMultiPassPlanningInternal_PartialPlanFailure(t *testing.T) {
	logger := &mockLogger{}
	runPlanManagerCalled := false

	instances := map[string]*orchestrator.Instance{
		"p1": {ID: "p1", WorktreePath: "/tmp/p1"},
		"p2": {ID: "p2", WorktreePath: "/tmp/p2"},
		"p3": {ID: "p3", WorktreePath: "/tmp/p3"},
	}

	// p1 succeeds, p2 fails, p3 succeeds
	parseResults := map[string]struct {
		plan *orchestrator.PlanSpec
		err  error
	}{
		"/tmp/p1": {plan: &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "t1"}}}, err: nil},
		"/tmp/p2": {plan: nil, err: errors.New("parse failed")},
		"/tmp/p3": {plan: &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "t3"}}}, err: nil},
	}

	deps := multiPassResumeDeps{
		getInstance: func(id string) *orchestrator.Instance {
			return instances[id]
		},
		isTmuxRunning: func(id string) bool {
			return false // All completed
		},
		saveSession: func() error { return nil },
		runPlanning: func() error { return nil },
		runPlanManager: func() error {
			runPlanManagerCalled = true
			return nil
		},
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			result := parseResults[worktreePath]
			return result.plan, result.err
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{"p1", "p2", "p3"},
		Objective:          "test objective",
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !runPlanManagerCalled {
		t.Error("expected RunPlanManager to be called with partial success")
	}

	// Verify that only 2 plans were collected (p1 and p3)
	validPlans := 0
	for _, p := range ultraSession.CandidatePlans {
		if p != nil {
			validPlans++
		}
	}
	if validPlans != 2 {
		t.Errorf("expected 2 valid plans, got %d", validPlans)
	}

	// Verify warning was logged for failed parse
	foundParseWarning := false
	for _, call := range logger.warnCalls {
		if call.msg == "failed to parse plan from completed planner" {
			foundParseWarning = true
			break
		}
	}
	if !foundParseWarning {
		t.Error("expected warning to be logged for failed plan parse")
	}
}

// TestResumeMultiPassPlanningInternal_RunPlanManagerError tests that errors
// from RunPlanManager are properly propagated.
func TestResumeMultiPassPlanningInternal_RunPlanManagerError(t *testing.T) {
	logger := &mockLogger{}

	instances := map[string]*orchestrator.Instance{
		"p1": {ID: "p1", WorktreePath: "/tmp/p1"},
	}

	deps := multiPassResumeDeps{
		getInstance: func(id string) *orchestrator.Instance {
			return instances[id]
		},
		isTmuxRunning: func(id string) bool {
			return false
		},
		saveSession: func() error { return nil },
		runPlanning: func() error { return nil },
		runPlanManager: func() error {
			return errors.New("evaluator failed to start")
		},
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			return &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "t1"}}}, nil
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{"p1"},
		Objective:          "test objective",
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err == nil {
		t.Error("expected error from RunPlanManager")
	}
	if err.Error() != "failed to start plan evaluator: evaluator failed to start" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestResumeMultiPassPlanningInternal_SaveSessionError tests that errors
// from SaveSession are logged but don't fail the operation.
func TestResumeMultiPassPlanningInternal_SaveSessionError(t *testing.T) {
	logger := &mockLogger{}
	runPlanManagerCalled := false

	instances := map[string]*orchestrator.Instance{
		"p1": {ID: "p1", WorktreePath: "/tmp/p1"},
	}

	deps := multiPassResumeDeps{
		getInstance: func(id string) *orchestrator.Instance {
			return instances[id]
		},
		isTmuxRunning: func(id string) bool {
			return false
		},
		saveSession: func() error {
			return errors.New("save failed")
		},
		runPlanning: func() error { return nil },
		runPlanManager: func() error {
			runPlanManagerCalled = true
			return nil
		},
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			return &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "t1"}}}, nil
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{"p1"},
		Objective:          "test objective",
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	// Should succeed despite save error
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !runPlanManagerCalled {
		t.Error("expected RunPlanManager to be called")
	}

	// Verify warning was logged
	foundSaveWarning := false
	for _, call := range logger.warnCalls {
		if call.msg == "failed to save session after triggering evaluator" {
			foundSaveWarning = true
			break
		}
	}
	if !foundSaveWarning {
		t.Error("expected warning to be logged for save session failure")
	}
}

// TestResumeMultiPassPlanningInternal_AlreadyProcessed tests that previously
// processed planners are not re-processed.
func TestResumeMultiPassPlanningInternal_AlreadyProcessed(t *testing.T) {
	logger := &mockLogger{}
	parseCallCount := 0

	instances := map[string]*orchestrator.Instance{
		"p1": {ID: "p1", WorktreePath: "/tmp/p1"},
		"p2": {ID: "p2", WorktreePath: "/tmp/p2"},
		"p3": {ID: "p3", WorktreePath: "/tmp/p3"},
	}

	deps := multiPassResumeDeps{
		getInstance: func(id string) *orchestrator.Instance {
			return instances[id]
		},
		isTmuxRunning: func(id string) bool {
			return false
		},
		saveSession: func() error { return nil },
		runPlanning: func() error { return nil },
		runPlanManager: func() error {
			return nil
		},
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			parseCallCount++
			return &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "t1"}}}, nil
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{"p1", "p2", "p3"},
		Objective:          "test objective",
		// p1 and p2 already processed
		ProcessedCoordinators: map[int]bool{
			0: true,
			1: true,
		},
		CandidatePlans: []*orchestrator.PlanSpec{
			{Tasks: []orchestrator.PlannedTask{{ID: "existing1"}}},
			{Tasks: []orchestrator.PlannedTask{{ID: "existing2"}}},
			nil, // p3 not yet collected
		},
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Only p3 should have been parsed
	if parseCallCount != 1 {
		t.Errorf("expected 1 parse call (for p3 only), got %d", parseCallCount)
	}
}

// TestResumeMultiPassPlanningInternal_CandidatePlansExpansion tests that
// CandidatePlans slice is properly expanded when it's smaller than needed.
func TestResumeMultiPassPlanningInternal_CandidatePlansExpansion(t *testing.T) {
	logger := &mockLogger{}

	instances := map[string]*orchestrator.Instance{
		"p1": {ID: "p1", WorktreePath: "/tmp/p1"},
		"p2": {ID: "p2", WorktreePath: "/tmp/p2"},
		"p3": {ID: "p3", WorktreePath: "/tmp/p3"},
	}

	deps := multiPassResumeDeps{
		getInstance: func(id string) *orchestrator.Instance {
			return instances[id]
		},
		isTmuxRunning: func(id string) bool {
			return false
		},
		saveSession: func() error { return nil },
		runPlanning: func() error { return nil },
		runPlanManager: func() error {
			return nil
		},
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			return &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "t1"}}}, nil
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{"p1", "p2", "p3"},
		Objective:          "test objective",
		CandidatePlans:     []*orchestrator.PlanSpec{}, // Empty - needs expansion
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(ultraSession.CandidatePlans) != 3 {
		t.Errorf("expected CandidatePlans to be expanded to 3, got %d", len(ultraSession.CandidatePlans))
	}
}

// TestResumeMultiPassPlanningInternal_RunPlanningError tests that errors
// from RunPlanning are properly propagated when starting fresh.
func TestResumeMultiPassPlanningInternal_RunPlanningError(t *testing.T) {
	logger := &mockLogger{}

	deps := multiPassResumeDeps{
		getInstance:   func(id string) *orchestrator.Instance { return nil },
		isTmuxRunning: func(id string) bool { return false },
		saveSession:   func() error { return nil },
		runPlanning: func() error {
			return errors.New("planning failed")
		},
		runPlanManager: func() error { return nil },
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			return nil, nil
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{}, // No planners - will trigger RunPlanning
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err == nil {
		t.Error("expected error from RunPlanning")
	}
	if err.Error() != "planning failed" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestResumeMultiPassPlanningInternal_AllNilInstances tests that when
// ALL instances return nil, the evaluator still gets triggered but fails
// with no valid plans.
func TestResumeMultiPassPlanningInternal_AllNilInstances(t *testing.T) {
	logger := &mockLogger{}

	deps := multiPassResumeDeps{
		getInstance: func(id string) *orchestrator.Instance {
			return nil // All return nil
		},
		isTmuxRunning: func(id string) bool {
			return false
		},
		saveSession: func() error { return nil },
		runPlanning: func() error { return nil },
		runPlanManager: func() error {
			t.Error("RunPlanManager should not be called with no valid plans")
			return nil
		},
		parsePlan: func(worktreePath, objective string) (*orchestrator.PlanSpec, error) {
			t.Error("parsePlan should not be called for nil instances")
			return nil, nil
		},
		logWarn: logger.warn,
		logInfo: logger.info,
	}

	ultraSession := &orchestrator.UltraPlanSession{
		Phase:              orchestrator.PhasePlanning,
		Config:             orchestrator.UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: []string{"p1", "p2", "p3"},
		Objective:          "test objective",
	}

	err := resumeMultiPassPlanningInternal(deps, ultraSession)

	if err == nil {
		t.Error("expected error when all instances are nil")
	}
	if err.Error() != "all multi-pass planners completed but no valid plans were produced" {
		t.Errorf("unexpected error message: %v", err)
	}

	// All should be marked as processed
	if len(ultraSession.ProcessedCoordinators) != 3 {
		t.Errorf("expected 3 processed coordinators, got %d", len(ultraSession.ProcessedCoordinators))
	}
}
