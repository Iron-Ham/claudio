package decomposition

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// createTestRepo creates a temporary directory structure simulating a Go repository.
func createTestRepo(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "decomposition-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create a simple Go project structure
	dirs := []string{
		"cmd/app",
		"internal/config",
		"internal/core",
		"internal/api",
		"pkg/utils",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create Go files with package declarations and imports
	files := map[string]string{
		"cmd/app/main.go": `package main

import (
	"github.com/example/project/internal/core"
	"github.com/example/project/internal/api"
)

func main() {
	core.Start()
	api.Serve()
}
`,
		"internal/config/config.go": `package config

type Config struct {
	Port int
}
`,
		"internal/core/core.go": `package core

import "github.com/example/project/internal/config"

func Start() {
	_ = config.Config{}
}
`,
		"internal/core/types.go": `package core

type Service interface {
	Run() error
}
`,
		"internal/api/api.go": `package api

import (
	"github.com/example/project/internal/core"
	"github.com/example/project/internal/config"
)

func Serve() {
	_ = core.Service(nil)
	_ = config.Config{}
}
`,
		"pkg/utils/utils.go": `package utils

func Helper() {}
`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}

	return tmpDir
}

func TestNewAnalyzer(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	analyzer := NewAnalyzer(tmpDir)
	if analyzer == nil {
		t.Fatal("NewAnalyzer returned nil")
	}

	if analyzer.repoPath != tmpDir {
		t.Errorf("repoPath = %q, want %q", analyzer.repoPath, tmpDir)
	}
}

func TestNewAnalyzerWithConfig(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	config := AnalyzerConfig{
		EnableGitHistory:              false,
		InferDependencies:             true,
		DependencyConfidenceThreshold: 80,
		RiskThresholds: RiskThresholds{
			Low:    20,
			Medium: 40,
			High:   60,
		},
	}

	analyzer := NewAnalyzerWithConfig(tmpDir, config)
	if analyzer.config.RiskThresholds.Low != 20 {
		t.Errorf("config.RiskThresholds.Low = %d, want 20", analyzer.config.RiskThresholds.Low)
	}
}

func TestAnalyze_BasicPlan(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	analyzer := NewAnalyzer(tmpDir)

	plan := &ultraplan.PlanSpec{
		ID:        "test-plan",
		Objective: "Test objective",
		Tasks: []ultraplan.PlannedTask{
			{
				ID:            "task-1",
				Title:         "Update config",
				Description:   "Modify configuration",
				Files:         []string{"internal/config/config.go"},
				DependsOn:     []string{},
				EstComplexity: ultraplan.ComplexityLow,
			},
			{
				ID:            "task-2",
				Title:         "Update core",
				Description:   "Modify core logic",
				Files:         []string{"internal/core/core.go"},
				DependsOn:     []string{"task-1"},
				EstComplexity: ultraplan.ComplexityMedium,
			},
			{
				ID:            "task-3",
				Title:         "Update API",
				Description:   "Modify API layer",
				Files:         []string{"internal/api/api.go"},
				DependsOn:     []string{"task-2"},
				EstComplexity: ultraplan.ComplexityLow,
			},
		},
		DependencyGraph: map[string][]string{
			"task-1": {},
			"task-2": {"task-1"},
			"task-3": {"task-2"},
		},
		ExecutionOrder: [][]string{
			{"task-1"},
			{"task-2"},
			{"task-3"},
		},
	}

	analysis, err := analyzer.Analyze(plan)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if analysis == nil {
		t.Fatal("Analysis is nil")
	}

	// Check that all tasks were analyzed
	if len(analysis.TaskAnalyses) != 3 {
		t.Errorf("TaskAnalyses count = %d, want 3", len(analysis.TaskAnalyses))
	}

	// Verify task analysis exists for each task
	for _, task := range plan.Tasks {
		if _, ok := analysis.TaskAnalyses[task.ID]; !ok {
			t.Errorf("Missing analysis for task %s", task.ID)
		}
	}
}

func TestAnalyze_NilPlan(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	analyzer := NewAnalyzer(tmpDir)
	analysis, err := analyzer.Analyze(nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if analysis != nil {
		t.Error("Expected nil analysis for nil plan")
	}
}

func TestAnalyze_CriticalPath(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	analyzer := NewAnalyzer(tmpDir)

	// Create a plan with a clear critical path
	plan := &ultraplan.PlanSpec{
		ID: "critical-path-test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "a", Title: "Task A", Files: []string{}, DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
			{ID: "b", Title: "Task B", Files: []string{}, DependsOn: []string{"a"}, EstComplexity: ultraplan.ComplexityHigh},
			{ID: "c", Title: "Task C", Files: []string{}, DependsOn: []string{"a"}, EstComplexity: ultraplan.ComplexityLow},
			{ID: "d", Title: "Task D", Files: []string{}, DependsOn: []string{"b", "c"}, EstComplexity: ultraplan.ComplexityMedium},
		},
		DependencyGraph: map[string][]string{
			"a": {},
			"b": {"a"},
			"c": {"a"},
			"d": {"b", "c"},
		},
		ExecutionOrder: [][]string{{"a"}, {"b", "c"}, {"d"}},
	}

	analysis, err := analyzer.Analyze(plan)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// The critical path should go through the high-complexity task
	// Expected: a -> b -> d (because b is high complexity)
	if len(analysis.CriticalPath) == 0 {
		t.Error("Expected non-empty critical path")
	}

	// Critical path should end with d
	if len(analysis.CriticalPath) > 0 {
		last := analysis.CriticalPath[len(analysis.CriticalPath)-1]
		if last != "d" {
			t.Errorf("Critical path ends with %q, want 'd'", last)
		}
	}

	// Should have a positive critical path length
	if analysis.CriticalPathLength <= 0 {
		t.Error("Expected positive critical path length")
	}
}

func TestAnalyze_RiskScoring(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	analyzer := NewAnalyzer(tmpDir)

	plan := &ultraplan.PlanSpec{
		ID: "risk-test",
		Tasks: []ultraplan.PlannedTask{
			{
				ID:            "low-risk",
				Title:         "Simple task",
				Files:         []string{"pkg/utils/utils.go"},
				DependsOn:     []string{},
				EstComplexity: ultraplan.ComplexityLow,
			},
			{
				ID:            "high-risk",
				Title:         "Complex task",
				Files:         []string{"internal/core/core.go", "internal/core/types.go", "internal/api/api.go", "internal/config/config.go", "cmd/app/main.go", "pkg/utils/utils.go"},
				DependsOn:     []string{},
				EstComplexity: ultraplan.ComplexityHigh,
			},
		},
		DependencyGraph: map[string][]string{
			"low-risk":  {},
			"high-risk": {},
		},
		ExecutionOrder: [][]string{{"low-risk", "high-risk"}},
	}

	analysis, err := analyzer.Analyze(plan)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	lowRisk := analysis.TaskAnalyses["low-risk"]
	highRisk := analysis.TaskAnalyses["high-risk"]

	// High-risk task should have higher risk score
	if highRisk.RiskScore <= lowRisk.RiskScore {
		t.Errorf("High-risk score (%d) should be greater than low-risk score (%d)",
			highRisk.RiskScore, lowRisk.RiskScore)
	}

	// High-risk task should have multiple risk factors
	if len(highRisk.RiskFactors) == 0 {
		t.Error("Expected risk factors for high-risk task")
	}
}

func TestAnalyze_RiskDistribution(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	analyzer := NewAnalyzer(tmpDir)

	plan := &ultraplan.PlanSpec{
		ID: "distribution-test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{}, DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
			{ID: "t2", Title: "Task 2", Files: []string{}, DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
			{ID: "t3", Title: "Task 3", Files: []string{}, DependsOn: []string{}, EstComplexity: ultraplan.ComplexityMedium},
		},
		DependencyGraph: map[string][]string{
			"t1": {},
			"t2": {},
			"t3": {},
		},
		ExecutionOrder: [][]string{{"t1", "t2", "t3"}},
	}

	analysis, err := analyzer.Analyze(plan)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	total := analysis.RiskDistribution.Low + analysis.RiskDistribution.Medium +
		analysis.RiskDistribution.High + analysis.RiskDistribution.Critical

	if total != 3 {
		t.Errorf("Risk distribution total = %d, want 3", total)
	}
}

func TestAnalyze_InferredDependencies_SharedFiles(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	config := DefaultAnalyzerConfig()
	config.DependencyConfidenceThreshold = 50 // Lower threshold to capture more inferences
	analyzer := NewAnalyzerWithConfig(tmpDir, config)

	// Create a plan where two tasks share files
	plan := &ultraplan.PlanSpec{
		ID: "shared-files-test",
		Tasks: []ultraplan.PlannedTask{
			{
				ID:            "task-a",
				Title:         "Modify config",
				Files:         []string{"internal/config/config.go"},
				DependsOn:     []string{},
				EstComplexity: ultraplan.ComplexityLow,
			},
			{
				ID:            "task-b",
				Title:         "Also modify config",
				Files:         []string{"internal/config/config.go"}, // Same file!
				DependsOn:     []string{},
				EstComplexity: ultraplan.ComplexityLow,
			},
		},
		DependencyGraph: map[string][]string{
			"task-a": {},
			"task-b": {},
		},
		ExecutionOrder: [][]string{{"task-a", "task-b"}},
	}

	analysis, err := analyzer.Analyze(plan)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should infer a dependency due to shared file
	foundSharedFileDep := false
	for _, dep := range analysis.InferredDependencies {
		if dep.Type == DependencyTypeSharedFile {
			foundSharedFileDep = true
			if !dep.IsHard {
				t.Error("Shared file dependency should be hard")
			}
			break
		}
	}

	if !foundSharedFileDep {
		t.Error("Expected to infer shared file dependency")
	}
}

func TestAnalyze_Suggestions(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	analyzer := NewAnalyzer(tmpDir)

	// Create a plan with issues that should trigger suggestions
	plan := &ultraplan.PlanSpec{
		ID: "suggestions-test",
		Tasks: []ultraplan.PlannedTask{
			{
				ID:            "complex-task",
				Title:         "Very complex task",
				Files:         []string{"internal/core/core.go", "internal/core/types.go", "internal/api/api.go", "internal/config/config.go", "cmd/app/main.go", "pkg/utils/utils.go"},
				DependsOn:     []string{},
				EstComplexity: ultraplan.ComplexityHigh,
			},
		},
		DependencyGraph: map[string][]string{
			"complex-task": {},
		},
		ExecutionOrder: [][]string{{"complex-task"}},
	}

	analysis, err := analyzer.Analyze(plan)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should suggest splitting the high-complexity, high-risk task
	if len(analysis.Suggestions) == 0 {
		t.Error("Expected suggestions for complex high-risk task")
	}

	// Check for split suggestion
	hasSplitSuggestion := false
	for _, s := range analysis.Suggestions {
		if s.Type == SuggestionSplitTask || s.Type == SuggestionIsolateRisky {
			hasSplitSuggestion = true
			break
		}
	}

	if !hasSplitSuggestion {
		t.Error("Expected split or isolate suggestion for complex task")
	}
}

func TestApply_EnhancesPlan(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	analyzer := NewAnalyzer(tmpDir)

	plan := &ultraplan.PlanSpec{
		ID: "apply-test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"internal/config/config.go"}, DependsOn: []string{}, Priority: 0},
			{ID: "t2", Title: "Task 2", Files: []string{"internal/config/config.go"}, DependsOn: []string{}, Priority: 0}, // Shared file
		},
		DependencyGraph: map[string][]string{
			"t1": {},
			"t2": {},
		},
		ExecutionOrder: [][]string{{"t1", "t2"}},
	}

	analysis, err := analyzer.Analyze(plan)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	enhanced := analysis.Apply(plan)
	if enhanced == nil {
		t.Fatal("Apply returned nil")
	}

	// The enhanced plan should preserve the original structure
	if len(enhanced.Tasks) != len(plan.Tasks) {
		t.Errorf("Task count changed: %d -> %d", len(plan.Tasks), len(enhanced.Tasks))
	}

	// IDs should be preserved
	if enhanced.ID != plan.ID {
		t.Errorf("Plan ID changed: %q -> %q", plan.ID, enhanced.ID)
	}
}

func TestApply_NilAnalysis(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "nil-test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1"},
		},
	}

	var analysis *Analysis = nil
	result := analysis.Apply(plan)

	if result != plan {
		t.Error("Expected same plan when analysis is nil")
	}
}

func TestGetEnhancedExecutionOrder(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	analyzer := NewAnalyzer(tmpDir)

	plan := &ultraplan.PlanSpec{
		ID: "exec-order-test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Low risk", Files: []string{}, DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
			{ID: "t2", Title: "Also low", Files: []string{}, DependsOn: []string{}, EstComplexity: ultraplan.ComplexityLow},
			{ID: "t3", Title: "Depends", Files: []string{}, DependsOn: []string{"t1", "t2"}, EstComplexity: ultraplan.ComplexityMedium},
		},
		DependencyGraph: map[string][]string{
			"t1": {},
			"t2": {},
			"t3": {"t1", "t2"},
		},
		ExecutionOrder: [][]string{{"t1", "t2"}, {"t3"}},
	}

	analysis, err := analyzer.Analyze(plan)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	enhanced := analysis.GetEnhancedExecutionOrder(plan)
	if enhanced == nil {
		t.Fatal("GetEnhancedExecutionOrder returned nil")
	}

	if len(enhanced.Groups) != 2 {
		t.Errorf("Group count = %d, want 2", len(enhanced.Groups))
	}

	// First group should have 2 tasks
	if len(enhanced.Groups[0].Tasks) != 2 {
		t.Errorf("First group task count = %d, want 2", len(enhanced.Groups[0].Tasks))
	}
}

func TestRiskLevel(t *testing.T) {
	tests := []struct {
		score int
		want  RiskLevel
	}{
		{0, RiskLevelLow},
		{10, RiskLevelLow},
		{24, RiskLevelLow},
		{25, RiskLevelMedium},
		{30, RiskLevelMedium},
		{49, RiskLevelMedium},
		{50, RiskLevelHigh},
		{60, RiskLevelHigh},
		{74, RiskLevelHigh},
		{75, RiskLevelCritical},
		{100, RiskLevelCritical},
	}

	analyzer := NewAnalyzer("")
	for _, tt := range tests {
		t.Run("score-"+strconv.Itoa(tt.score), func(t *testing.T) {
			got := analyzer.scoreToRiskLevel(tt.score)
			if got != tt.want {
				t.Errorf("scoreToRiskLevel(%d) = %q, want %q", tt.score, got, tt.want)
			}
		})
	}
}

func TestDefaultAnalyzerConfig(t *testing.T) {
	config := DefaultAnalyzerConfig()

	if !config.EnableGitHistory {
		t.Error("Expected EnableGitHistory to be true by default")
	}

	if !config.InferDependencies {
		t.Error("Expected InferDependencies to be true by default")
	}

	if config.DependencyConfidenceThreshold != 70 {
		t.Errorf("DependencyConfidenceThreshold = %d, want 70", config.DependencyConfidenceThreshold)
	}

	if config.RiskThresholds.Low != 25 {
		t.Errorf("RiskThresholds.Low = %d, want 25", config.RiskThresholds.Low)
	}
}

func TestRiskFactor_Constants(t *testing.T) {
	// Ensure constants are defined correctly
	if RiskFactorFileCount == "" {
		t.Error("RiskFactorFileCount should not be empty")
	}
	if RiskFactorComplexity == "" {
		t.Error("RiskFactorComplexity should not be empty")
	}
	if RiskFactorCentrality == "" {
		t.Error("RiskFactorCentrality should not be empty")
	}
}

func TestDependencyType_Constants(t *testing.T) {
	types := []DependencyType{
		DependencyTypeImport,
		DependencyTypeInterface,
		DependencyTypeSamePackage,
		DependencyTypeSharedFile,
		DependencyTypeTestDependency,
	}

	for _, dt := range types {
		if dt == "" {
			t.Error("DependencyType constant should not be empty")
		}
	}
}

func TestSuggestionType_Constants(t *testing.T) {
	types := []SuggestionType{
		SuggestionSplitTask,
		SuggestionMergeTasks,
		SuggestionAddDependency,
		SuggestionRemoveDependency,
		SuggestionReorderTasks,
		SuggestionIsolateRisky,
		SuggestionParallelize,
	}

	for _, st := range types {
		if st == "" {
			t.Error("SuggestionType constant should not be empty")
		}
	}
}

func TestTaskAnalysis_IsCriticalPath(t *testing.T) {
	tmpDir := createTestRepo(t)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	analyzer := NewAnalyzer(tmpDir)

	// Linear dependency chain - all tasks should be on critical path
	plan := &ultraplan.PlanSpec{
		ID: "critical-path-marking",
		Tasks: []ultraplan.PlannedTask{
			{ID: "a", Title: "A", Files: []string{}, DependsOn: []string{}, EstComplexity: ultraplan.ComplexityMedium},
			{ID: "b", Title: "B", Files: []string{}, DependsOn: []string{"a"}, EstComplexity: ultraplan.ComplexityMedium},
			{ID: "c", Title: "C", Files: []string{}, DependsOn: []string{"b"}, EstComplexity: ultraplan.ComplexityMedium},
		},
		DependencyGraph: map[string][]string{
			"a": {},
			"b": {"a"},
			"c": {"b"},
		},
		ExecutionOrder: [][]string{{"a"}, {"b"}, {"c"}},
	}

	analysis, err := analyzer.Analyze(plan)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// In a linear chain, all tasks should be on the critical path
	for _, taskID := range []string{"a", "b", "c"} {
		if ta, ok := analysis.TaskAnalyses[taskID]; ok {
			if !ta.IsCriticalPath {
				t.Errorf("Task %q should be on critical path", taskID)
			}
		} else {
			t.Errorf("Missing analysis for task %q", taskID)
		}
	}
}
