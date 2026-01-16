package decomposition

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
	"github.com/Iron-Ham/claudio/internal/worktree"
)

// Analyzer performs code structure analysis to enhance task decomposition.
type Analyzer struct {
	repoPath string
	config   AnalyzerConfig

	// Cached analysis results
	packageGraph *PackageGraph
	filePackages map[string]string // file path -> package path
}

// NewAnalyzer creates a new decomposition analyzer for the given repository.
func NewAnalyzer(repoPath string) *Analyzer {
	return &Analyzer{
		repoPath:     repoPath,
		config:       DefaultAnalyzerConfig(),
		filePackages: make(map[string]string),
	}
}

// NewAnalyzerWithConfig creates an analyzer with custom configuration.
func NewAnalyzerWithConfig(repoPath string, config AnalyzerConfig) *Analyzer {
	return &Analyzer{
		repoPath:     repoPath,
		config:       config,
		filePackages: make(map[string]string),
	}
}

// Analyze performs comprehensive analysis of a plan and returns enhancement recommendations.
func (a *Analyzer) Analyze(plan *ultraplan.PlanSpec) (*Analysis, error) {
	if plan == nil {
		return nil, nil
	}

	// Build package graph from codebase
	if err := a.buildPackageGraph(); err != nil {
		// Non-fatal - continue without package analysis
		a.packageGraph = nil
	}

	analysis := &Analysis{
		TaskAnalyses:         make(map[string]*TaskAnalysis),
		InferredDependencies: []InferredDependency{},
		Suggestions:          []Suggestion{},
		PackageGraph:         a.packageGraph,
	}

	// Build reverse dependency map for impact scoring
	reverseDeps := a.buildReverseDependencyMap(plan)

	// Analyze each task
	for i := range plan.Tasks {
		task := &plan.Tasks[i]
		taskAnalysis := a.analyzeTask(task, plan, reverseDeps)
		analysis.TaskAnalyses[task.ID] = taskAnalysis

		// Update risk distribution
		switch taskAnalysis.RiskLevel {
		case RiskLevelLow:
			analysis.RiskDistribution.Low++
		case RiskLevelMedium:
			analysis.RiskDistribution.Medium++
		case RiskLevelHigh:
			analysis.RiskDistribution.High++
		case RiskLevelCritical:
			analysis.RiskDistribution.Critical++
		}
	}

	// Infer dependencies from code analysis
	if a.config.InferDependencies {
		analysis.InferredDependencies = a.inferDependencies(plan)
	}

	// Compute critical path
	analysis.CriticalPath, analysis.CriticalPathLength = a.computeCriticalPath(plan, analysis.TaskAnalyses)

	// Mark tasks on critical path with position
	for i, taskID := range analysis.CriticalPath {
		if ta, ok := analysis.TaskAnalyses[taskID]; ok {
			ta.IsCriticalPath = true
			ta.CriticalPathPosition = i + 1
		}
	}

	// Calculate parallelism metrics
	analysis.ParallelismScore = a.calculateParallelismScore(plan)
	analysis.AverageGroupSize = a.calculateAverageGroupSize(plan)
	analysis.BottleneckGroups = a.findBottleneckGroups(plan)

	// Find file conflict clusters
	analysis.FileConflictClusters = a.findFileConflictClusters(plan)

	// Find package hotspots
	analysis.PackageHotspots = a.findPackageHotspots(plan)

	// Perform transitive reduction if enabled
	if a.config.EnableTransitiveReduction {
		analysis.OptimizedDependencies, analysis.RemovedDependencies = a.performTransitiveReduction(plan)
	}

	// Generate optimized execution order
	analysis.OptimizedExecutionOrder = a.optimizeExecutionOrder(plan, analysis.TaskAnalyses)

	// Generate suggestions
	analysis.Suggestions = a.generateSuggestions(plan, analysis)

	return analysis, nil
}

// buildPackageGraph analyzes the Go codebase to build a package dependency graph.
func (a *Analyzer) buildPackageGraph() error {
	a.packageGraph = &PackageGraph{
		Packages: make(map[string]*PackageInfo),
		Edges:    make(map[string][]string),
	}

	// Walk the repository looking for Go files
	err := filepath.Walk(a.repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip hidden directories and vendor
		name := info.Name()
		if info.IsDir() {
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			// Skip git submodule directories to avoid errors when walking
			// uninitialized or partially initialized submodules
			if worktree.IsSubmoduleDir(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process Go files
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}

		// Parse the file to extract package and imports
		a.parseGoFile(path)
		return nil
	})

	if err != nil {
		return err
	}

	// Calculate centrality for each package
	a.calculatePackageCentrality()

	return nil
}

// parseGoFile extracts package and import information from a Go file.
func (a *Analyzer) parseGoFile(filePath string) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return
	}

	// Determine package path
	relPath, err := filepath.Rel(a.repoPath, filePath)
	if err != nil {
		return
	}
	dir := filepath.Dir(relPath)
	pkgPath := strings.ReplaceAll(dir, string(filepath.Separator), "/")

	// Track file -> package mapping
	a.filePackages[relPath] = pkgPath

	// Create or update package info
	pkgInfo, exists := a.packageGraph.Packages[pkgPath]
	if !exists {
		pkgInfo = &PackageInfo{
			Path:       pkgPath,
			Name:       node.Name.Name,
			Files:      []string{},
			ImportedBy: []string{},
			Imports:    []string{},
			IsInternal: strings.Contains(pkgPath, "internal"),
		}
		a.packageGraph.Packages[pkgPath] = pkgInfo
	}
	pkgInfo.Files = append(pkgInfo.Files, relPath)

	// Extract imports
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// Only track internal imports (same project)
		if !strings.Contains(importPath, ".") {
			continue
		}

		// Add to edges
		if !slices.Contains(a.packageGraph.Edges[pkgPath], importPath) {
			a.packageGraph.Edges[pkgPath] = append(a.packageGraph.Edges[pkgPath], importPath)
		}

		// Track in package imports
		if !slices.Contains(pkgInfo.Imports, importPath) {
			pkgInfo.Imports = append(pkgInfo.Imports, importPath)
		}

		// Update ImportedBy for the target package
		if targetPkg, ok := a.packageGraph.Packages[importPath]; ok {
			if !slices.Contains(targetPkg.ImportedBy, pkgPath) {
				targetPkg.ImportedBy = append(targetPkg.ImportedBy, pkgPath)
			}
		}
	}
}

// calculatePackageCentrality computes how central each package is in the dependency graph.
func (a *Analyzer) calculatePackageCentrality() {
	if a.packageGraph == nil {
		return
	}

	// Simple centrality: number of packages that directly or indirectly depend on this package
	for pkgPath, pkgInfo := range a.packageGraph.Packages {
		dependents := a.countDependents(pkgPath, make(map[string]bool))
		total := len(a.packageGraph.Packages)
		if total > 1 {
			pkgInfo.Centrality = float64(dependents) / float64(total-1)
		}
	}
}

// countDependents recursively counts packages that depend on the given package.
func (a *Analyzer) countDependents(pkgPath string, visited map[string]bool) int {
	if visited[pkgPath] {
		return 0
	}
	visited[pkgPath] = true

	pkgInfo, ok := a.packageGraph.Packages[pkgPath]
	if !ok {
		return 0
	}

	count := len(pkgInfo.ImportedBy)
	for _, importer := range pkgInfo.ImportedBy {
		count += a.countDependents(importer, visited)
	}
	return count
}

// analyzeTask performs comprehensive analysis of a single task.
func (a *Analyzer) analyzeTask(task *ultraplan.PlannedTask, plan *ultraplan.PlanSpec, reverseDeps map[string][]string) *TaskAnalysis {
	analysis := &TaskAnalysis{
		TaskID:              task.ID,
		RiskFactors:         []RiskFactor{},
		AffectedPackages:    []string{},
		FileCategories:      make(map[string][]string),
		ParallelizationSafe: true, // Assume safe until proven otherwise
		FileAnalysis:        []FileAnalysis{},
	}

	// Analyze files and categorize them
	packageSet := make(map[string]bool)
	for _, file := range task.Files {
		fa := a.analyzeFile(file)
		analysis.FileAnalysis = append(analysis.FileAnalysis, fa)

		if fa.Package != "" {
			packageSet[fa.Package] = true
		}

		// Categorize file
		category := categorizeFile(file)
		analysis.FileCategories[category] = append(analysis.FileCategories[category], file)
	}

	for pkg := range packageSet {
		analysis.AffectedPackages = append(analysis.AffectedPackages, pkg)
	}

	// Calculate dependent counts
	directDeps := reverseDeps[task.ID]
	analysis.DependentCount = len(directDeps)
	analysis.TransitiveDependentCount = a.countTransitiveDependents(task.ID, reverseDeps)

	// Calculate risk factors
	analysis.RiskFactors = a.calculateRiskFactors(task, analysis, plan)

	// Sum up risk score
	for _, rf := range analysis.RiskFactors {
		analysis.RiskScore += rf.Score
	}

	// Cap risk score at 100
	if analysis.RiskScore > 100 {
		analysis.RiskScore = 100
	}

	// Determine risk level based on thresholds
	analysis.RiskLevel = a.scoreToRiskLevel(analysis.RiskScore)

	// Calculate centrality
	analysis.Centrality = a.calculateTaskCentrality(analysis.AffectedPackages)

	// Calculate impact score
	analysis.ImpactScore = a.calculateImpactScore(analysis.DependentCount, analysis.TransitiveDependentCount, len(plan.Tasks))

	// Calculate blocking score (combination of risk and impact)
	// Higher blocking score = should run early to fail fast or unblock others
	analysis.BlockingScore = (analysis.RiskScore*40 + analysis.ImpactScore*60) / 100

	// Determine parallelization safety
	analysis.ParallelizationSafe = a.isParallelizationSafe(task, plan, analysis)

	// Calculate suggested priority
	// Higher risk = lower number = higher priority (run earlier)
	analysis.SuggestedPriority = task.Priority - (analysis.RiskScore / 10)

	// Generate split suggestions for complex tasks
	if task.EstComplexity == ultraplan.ComplexityHigh || len(task.Files) > 5 || analysis.RiskScore > 70 {
		analysis.SuggestedSplits = a.suggestTaskSplits(task, analysis)
	}

	return analysis
}

// analyzeFile analyzes a single file for risk and dependency information.
func (a *Analyzer) analyzeFile(filePath string) FileAnalysis {
	fa := FileAnalysis{
		Path:       filePath,
		IsTestFile: strings.HasSuffix(filePath, "_test.go"),
	}

	// Determine package
	if pkg, ok := a.filePackages[filePath]; ok {
		fa.Package = pkg
	} else {
		// Infer from path
		dir := filepath.Dir(filePath)
		fa.Package = strings.ReplaceAll(dir, string(filepath.Separator), "/")
	}

	// Check import count (how many packages import this file's package)
	if a.packageGraph != nil {
		if pkgInfo, ok := a.packageGraph.Packages[fa.Package]; ok {
			fa.ImportCount = len(pkgInfo.ImportedBy)
		}
	}

	// Check if file likely defines interfaces (simple heuristic based on filename)
	fa.IsInterface = strings.Contains(strings.ToLower(filePath), "interface") ||
		strings.Contains(strings.ToLower(filePath), "contract") ||
		strings.HasSuffix(filePath, "_iface.go")

	return fa
}

// calculateRiskFactors determines risk factors for a task.
func (a *Analyzer) calculateRiskFactors(task *ultraplan.PlannedTask, taskAnalysis *TaskAnalysis, plan *ultraplan.PlanSpec) []RiskFactor {
	var factors []RiskFactor

	// File count risk
	fileCount := len(task.Files)
	if fileCount > 5 {
		factors = append(factors, RiskFactor{
			Name:        RiskFactorFileCount,
			Description: "Task modifies many files",
			Score:       min(25, fileCount*2),
			Mitigation:  "Consider splitting into smaller tasks",
		})
	}

	// Complexity risk
	switch task.EstComplexity {
	case ultraplan.ComplexityHigh:
		factors = append(factors, RiskFactor{
			Name:        RiskFactorComplexity,
			Description: "High estimated complexity",
			Score:       20,
			Mitigation:  "Break down into smaller, focused tasks",
		})
	case ultraplan.ComplexityMedium:
		factors = append(factors, RiskFactor{
			Name:        RiskFactorComplexity,
			Description: "Medium estimated complexity",
			Score:       10,
		})
	case ultraplan.ComplexityLow:
		// Low complexity adds no risk
	}

	// Core package risk
	for _, pkg := range taskAnalysis.AffectedPackages {
		for _, pattern := range a.config.CorePackagePatterns {
			if strings.Contains(pkg, pattern) {
				factors = append(factors, RiskFactor{
					Name:        RiskFactorCorePackage,
					Description: "Modifies core package: " + pkg,
					Score:       15,
					Mitigation:  "Review changes carefully, ensure tests pass",
				})
				break
			}
		}
	}

	// Interface file risk
	for _, fa := range taskAnalysis.FileAnalysis {
		if fa.IsInterface {
			factors = append(factors, RiskFactor{
				Name:        RiskFactorInterfaceFile,
				Description: "Modifies interface definition: " + fa.Path,
				Score:       15,
				Mitigation:  "Changes may affect all implementations",
			})
			break
		}
	}

	// Centrality risk
	if taskAnalysis.Centrality > 0.5 {
		factors = append(factors, RiskFactor{
			Name:        RiskFactorCentrality,
			Description: "Modifies highly central code",
			Score:       int(taskAnalysis.Centrality * 20),
			Mitigation:  "Many other parts of the codebase depend on this code",
		})
	}

	// Shared files risk
	sharedFiles := a.findSharedFiles(task, plan)
	if len(sharedFiles) > 0 {
		factors = append(factors, RiskFactor{
			Name:        RiskFactorSharedFiles,
			Description: "Files shared with other tasks: " + strings.Join(sharedFiles, ", "),
			Score:       len(sharedFiles) * 10,
			Mitigation:  "Coordinate with dependent tasks or add explicit dependencies",
		})
	}

	// Cross-package risk
	if len(taskAnalysis.AffectedPackages) > 2 {
		factors = append(factors, RiskFactor{
			Name:        RiskFactorCrossPackage,
			Description: "Task spans multiple packages",
			Score:       (len(taskAnalysis.AffectedPackages) - 2) * 5,
			Mitigation:  "Consider splitting by package boundary",
		})
	}

	// High import fan-out risk
	maxImporters := 0
	for _, fa := range taskAnalysis.FileAnalysis {
		if fa.ImportCount > maxImporters {
			maxImporters = fa.ImportCount
		}
	}
	if maxImporters > 5 {
		factors = append(factors, RiskFactor{
			Name:        RiskFactorDependencyFan,
			Description: "Code is imported by many packages",
			Score:       min(20, maxImporters*2),
			Mitigation:  "Changes will affect many dependents",
		})
	}

	return factors
}

// findSharedFiles finds files that this task shares with other tasks in the plan.
func (a *Analyzer) findSharedFiles(task *ultraplan.PlannedTask, plan *ultraplan.PlanSpec) []string {
	fileSet := make(map[string]bool)
	for _, f := range task.Files {
		fileSet[f] = true
	}

	var shared []string
	for _, other := range plan.Tasks {
		if other.ID == task.ID {
			continue
		}
		for _, f := range other.Files {
			if fileSet[f] {
				shared = append(shared, f)
			}
		}
	}
	return shared
}

// scoreToRiskLevel converts a numeric risk score to a risk level.
func (a *Analyzer) scoreToRiskLevel(score int) RiskLevel {
	if score < a.config.RiskThresholds.Low {
		return RiskLevelLow
	}
	if score < a.config.RiskThresholds.Medium {
		return RiskLevelMedium
	}
	if score < a.config.RiskThresholds.High {
		return RiskLevelHigh
	}
	return RiskLevelCritical
}

// calculateTaskCentrality determines how central this task's changes are.
func (a *Analyzer) calculateTaskCentrality(packages []string) float64 {
	if a.packageGraph == nil || len(packages) == 0 {
		return 0
	}

	maxCentrality := 0.0
	for _, pkg := range packages {
		if pkgInfo, ok := a.packageGraph.Packages[pkg]; ok {
			if pkgInfo.Centrality > maxCentrality {
				maxCentrality = pkgInfo.Centrality
			}
		}
	}
	return maxCentrality
}

// isParallelizationSafe determines if a task is safe to run in parallel.
func (a *Analyzer) isParallelizationSafe(task *ultraplan.PlannedTask, plan *ultraplan.PlanSpec, analysis *TaskAnalysis) bool {
	// Not safe if high risk
	if analysis.RiskLevel == RiskLevelCritical {
		return false
	}

	// Not safe if modifies files shared with non-dependent tasks
	for _, other := range plan.Tasks {
		if other.ID == task.ID {
			continue
		}
		// Skip if there's already a dependency relationship
		if slices.Contains(task.DependsOn, other.ID) || slices.Contains(other.DependsOn, task.ID) {
			continue
		}

		// Check for file overlap
		for _, f1 := range task.Files {
			if slices.Contains(other.Files, f1) {
				return false
			}
		}
	}

	return true
}

// inferDependencies analyzes code structure to infer task dependencies.
func (a *Analyzer) inferDependencies(plan *ultraplan.PlanSpec) []InferredDependency {
	var deps []InferredDependency

	// Build task -> packages map
	taskPackages := make(map[string][]string)
	for _, task := range plan.Tasks {
		packages := make(map[string]bool)
		for _, file := range task.Files {
			if pkg, ok := a.filePackages[file]; ok {
				packages[pkg] = true
			}
		}
		for pkg := range packages {
			taskPackages[task.ID] = append(taskPackages[task.ID], pkg)
		}
	}

	// Check for same-package dependencies
	for _, task1 := range plan.Tasks {
		for _, task2 := range plan.Tasks {
			if task1.ID >= task2.ID { // Avoid duplicates and self-comparison
				continue
			}

			// Already have explicit dependency?
			if slices.Contains(task1.DependsOn, task2.ID) || slices.Contains(task2.DependsOn, task1.ID) {
				continue
			}

			// Check for shared packages
			for _, pkg1 := range taskPackages[task1.ID] {
				if slices.Contains(taskPackages[task2.ID], pkg1) {
					// Tasks share a package - infer soft dependency
					deps = append(deps, InferredDependency{
						FromTaskID: task1.ID,
						ToTaskID:   task2.ID,
						Reason:     "Both tasks modify package: " + pkg1,
						Confidence: 60,
						Type:       DependencyTypeSamePackage,
						IsHard:     false,
					})
					break
				}
			}

			// Check for shared files
			for _, f1 := range task1.Files {
				if slices.Contains(task2.Files, f1) {
					deps = append(deps, InferredDependency{
						FromTaskID: task1.ID,
						ToTaskID:   task2.ID,
						Reason:     "Both tasks modify file: " + f1,
						Confidence: 90,
						Type:       DependencyTypeSharedFile,
						IsHard:     true, // Shared files require ordering
					})
					break
				}
			}

			// Check for import dependencies between packages
			if a.packageGraph != nil {
				for _, pkg1 := range taskPackages[task1.ID] {
					for _, pkg2 := range taskPackages[task2.ID] {
						if slices.Contains(a.packageGraph.Edges[pkg2], pkg1) {
							// pkg2 imports pkg1, so task2's package depends on task1's package
							deps = append(deps, InferredDependency{
								FromTaskID: task1.ID,
								ToTaskID:   task2.ID,
								Reason:     "Package " + pkg2 + " imports " + pkg1,
								Confidence: 70,
								Type:       DependencyTypeImport,
								IsHard:     false,
							})
						}
					}
				}
			}
		}
	}

	// Filter by confidence threshold
	var filtered []InferredDependency
	for _, dep := range deps {
		if dep.Confidence >= a.config.DependencyConfidenceThreshold {
			filtered = append(filtered, dep)
		}
	}

	return filtered
}

// computeCriticalPath finds the longest path through the task dependency graph.
func (a *Analyzer) computeCriticalPath(plan *ultraplan.PlanSpec, analyses map[string]*TaskAnalysis) ([]string, float64) {
	if plan == nil || len(plan.Tasks) == 0 {
		return nil, 0
	}

	// Build adjacency list for reverse traversal (task -> tasks that depend on it)
	dependents := make(map[string][]string)
	for _, task := range plan.Tasks {
		for _, dep := range task.DependsOn {
			dependents[dep] = append(dependents[dep], task.ID)
		}
	}

	// Weight function: task complexity
	weight := func(taskID string) float64 {
		if analysis, ok := analyses[taskID]; ok {
			// Base weight from complexity
			baseWeight := 1.0
			task := plan.GetTask(taskID)
			if task != nil {
				switch task.EstComplexity {
				case ultraplan.ComplexityLow:
					baseWeight = 1.0
				case ultraplan.ComplexityMedium:
					baseWeight = 2.0
				case ultraplan.ComplexityHigh:
					baseWeight = 4.0
				}
			}
			// Add risk factor
			return baseWeight + float64(analysis.RiskScore)/50.0
		}
		return 1.0
	}

	// Find all leaf tasks (no dependents)
	var endTasks []string
	for _, task := range plan.Tasks {
		if len(dependents[task.ID]) == 0 {
			endTasks = append(endTasks, task.ID)
		}
	}

	// DFS to find longest path from each root
	var longestPath []string
	longestLength := 0.0

	// Memoization for longest path to each node
	memo := make(map[string]struct {
		path   []string
		length float64
	})

	var findLongest func(taskID string) ([]string, float64)
	findLongest = func(taskID string) ([]string, float64) {
		if cached, ok := memo[taskID]; ok {
			return cached.path, cached.length
		}

		task := plan.GetTask(taskID)
		if task == nil {
			return nil, 0
		}

		nodeWeight := weight(taskID)

		// No dependencies - this is a start node
		if len(task.DependsOn) == 0 {
			path := []string{taskID}
			memo[taskID] = struct {
				path   []string
				length float64
			}{path, nodeWeight}
			return path, nodeWeight
		}

		// Find longest path among dependencies
		var bestPath []string
		bestLength := 0.0
		for _, dep := range task.DependsOn {
			depPath, depLength := findLongest(dep)
			if depLength > bestLength {
				bestLength = depLength
				bestPath = depPath
			}
		}

		// Add current task to path
		path := append([]string{}, bestPath...)
		path = append(path, taskID)
		length := bestLength + nodeWeight

		memo[taskID] = struct {
			path   []string
			length float64
		}{path, length}
		return path, length
	}

	// Find longest path ending at any leaf task
	for _, endTask := range endTasks {
		path, length := findLongest(endTask)
		if length > longestLength {
			longestLength = length
			longestPath = path
		}
	}

	// If no end tasks found, search all tasks
	if len(longestPath) == 0 {
		for _, task := range plan.Tasks {
			path, length := findLongest(task.ID)
			if length > longestLength {
				longestLength = length
				longestPath = path
			}
		}
	}

	return longestPath, longestLength
}

// generateSuggestions creates actionable recommendations based on analysis.
func (a *Analyzer) generateSuggestions(plan *ultraplan.PlanSpec, analysis *Analysis) []Suggestion {
	var suggestions []Suggestion

	// Suggest splitting high-complexity tasks
	for taskID, ta := range analysis.TaskAnalyses {
		if ta.RiskLevel == RiskLevelCritical || ta.RiskLevel == RiskLevelHigh {
			task := plan.GetTask(taskID)
			if task != nil && task.EstComplexity == ultraplan.ComplexityHigh {
				suggestions = append(suggestions, Suggestion{
					Type:          SuggestionSplitTask,
					Priority:      1,
					Title:         "Split high-risk task: " + task.Title,
					Description:   "Task has high risk score and complexity. Consider splitting into smaller, focused tasks.",
					AffectedTasks: []string{taskID},
					Impact:        "Reduces failure risk and improves parallelization",
				})
			}
		}
	}

	// Suggest isolating risky tasks
	if analysis.RiskDistribution.Critical > 0 {
		var criticalTasks []string
		for taskID, ta := range analysis.TaskAnalyses {
			if ta.RiskLevel == RiskLevelCritical {
				criticalTasks = append(criticalTasks, taskID)
			}
		}
		suggestions = append(suggestions, Suggestion{
			Type:          SuggestionIsolateRisky,
			Priority:      1,
			Title:         "Isolate critical-risk tasks",
			Description:   "Run critical tasks sequentially at the start to catch failures early.",
			AffectedTasks: criticalTasks,
			Impact:        "Early failure detection, reduced rework",
		})
	}

	// Suggest adding inferred dependencies
	for _, dep := range analysis.InferredDependencies {
		if dep.IsHard && dep.Confidence >= 80 {
			suggestions = append(suggestions, Suggestion{
				Type:          SuggestionAddDependency,
				Priority:      2,
				Title:         "Add dependency: " + dep.FromTaskID + " → " + dep.ToTaskID,
				Description:   dep.Reason,
				AffectedTasks: []string{dep.FromTaskID, dep.ToTaskID},
				Impact:        "Prevents merge conflicts or inconsistent state",
			})
		}
	}

	// Suggest parallelization opportunities
	for i, group := range plan.ExecutionOrder {
		if len(group) == 1 {
			// Only one task in group - check if it really needs to be alone
			taskID := group[0]
			ta, ok := analysis.TaskAnalyses[taskID]
			if ok && ta.ParallelizationSafe && ta.RiskLevel == RiskLevelLow {
				// Check if previous group exists and could include this task
				if i > 0 {
					suggestions = append(suggestions, Suggestion{
						Type:          SuggestionParallelize,
						Priority:      3,
						Title:         "Consider parallelizing: " + taskID,
						Description:   "This task is low-risk and parallelization-safe. Review if dependencies are truly necessary.",
						AffectedTasks: []string{taskID},
						Impact:        "Improved parallelism, faster execution",
					})
				}
			}
		}
	}

	// Warn about critical path bottlenecks
	if len(analysis.CriticalPath) > 3 {
		suggestions = append(suggestions, Suggestion{
			Type:          SuggestionReorderTasks,
			Priority:      2,
			Title:         "Long critical path detected",
			Description:   "The critical path contains " + strconv.Itoa(len(analysis.CriticalPath)) + " tasks. Consider if dependencies can be relaxed.",
			AffectedTasks: analysis.CriticalPath,
			Impact:        "Reducing critical path length speeds up overall execution",
		})
	}

	return suggestions
}

// GetPackageForFile returns the package path for a given file.
func (a *Analyzer) GetPackageForFile(filePath string) string {
	return a.filePackages[filePath]
}

// GetPackageGraph returns the analyzed package dependency graph.
func (a *Analyzer) GetPackageGraph() *PackageGraph {
	return a.packageGraph
}

// -----------------------------------------------------------------------------
// Helper Functions (merged from multiple implementations)
// -----------------------------------------------------------------------------

// buildReverseDependencyMap creates a map of task ID -> tasks that depend on it.
func (a *Analyzer) buildReverseDependencyMap(spec *ultraplan.PlanSpec) map[string][]string {
	reverseDeps := make(map[string][]string)
	for _, task := range spec.Tasks {
		for _, depID := range task.DependsOn {
			reverseDeps[depID] = append(reverseDeps[depID], task.ID)
		}
	}
	return reverseDeps
}

// countTransitiveDependents counts all tasks that directly or indirectly depend on a task.
func (a *Analyzer) countTransitiveDependents(taskID string, reverseDeps map[string][]string) int {
	visited := make(map[string]bool)
	var count func(id string)
	count = func(id string) {
		for _, depID := range reverseDeps[id] {
			if !visited[depID] {
				visited[depID] = true
				count(depID)
			}
		}
	}
	count(taskID)
	return len(visited)
}

// categorizeFile determines the category of a file based on its extension and name.
func categorizeFile(file string) string {
	ext := strings.ToLower(filepath.Ext(file))
	base := filepath.Base(file)

	switch {
	case strings.HasSuffix(base, "_test.go"):
		return "test"
	case ext == ".go":
		return "source"
	case ext == ".md" || ext == ".txt":
		return "documentation"
	case ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".toml":
		return "config"
	case ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx":
		return "frontend"
	case ext == ".css" || ext == ".scss" || ext == ".less":
		return "styles"
	case ext == ".html" || ext == ".tmpl":
		return "templates"
	case ext == ".sql":
		return "database"
	case ext == ".proto":
		return "proto"
	case ext == ".sh" || ext == ".bash":
		return "scripts"
	default:
		return "other"
	}
}

// calculateImpactScore computes how much a task affects the overall plan.
func (a *Analyzer) calculateImpactScore(directDeps, transitiveDeps, totalTasks int) int {
	if totalTasks <= 1 {
		return 0
	}

	// Direct dependents are weighted more heavily
	directWeight := float64(directDeps) / float64(totalTasks) * 50
	transitiveWeight := float64(transitiveDeps) / float64(totalTasks) * 50

	score := int(directWeight + transitiveWeight)
	if score > 100 {
		score = 100
	}
	return score
}

// suggestTaskSplits generates split suggestions for complex tasks.
func (a *Analyzer) suggestTaskSplits(task *ultraplan.PlannedTask, analysis *TaskAnalysis) []SplitSuggestion {
	var suggestions []SplitSuggestion

	// Split by file type if multiple categories
	if len(analysis.FileCategories) > 1 {
		var proposed []ProposedTask
		for category, files := range analysis.FileCategories {
			proposed = append(proposed, ProposedTask{
				Title:         task.Title + " - " + category + " files",
				Files:         files,
				EstComplexity: estimateComplexity(len(files)),
			})
		}
		if len(proposed) > 1 {
			suggestions = append(suggestions, SplitSuggestion{
				Reason:        "Task contains multiple file types",
				SplitBy:       "file-type",
				ProposedTasks: proposed,
			})
		}
	}

	// Split by package if multiple packages
	if len(analysis.AffectedPackages) > 2 {
		var proposed []ProposedTask
		for _, pkg := range analysis.AffectedPackages {
			var pkgFiles []string
			for _, fa := range analysis.FileAnalysis {
				if fa.Package == pkg {
					pkgFiles = append(pkgFiles, fa.Path)
				}
			}
			if len(pkgFiles) > 0 {
				proposed = append(proposed, ProposedTask{
					Title:         task.Title + " - " + filepath.Base(pkg),
					Files:         pkgFiles,
					EstComplexity: estimateComplexity(len(pkgFiles)),
				})
			}
		}
		if len(proposed) > 1 {
			suggestions = append(suggestions, SplitSuggestion{
				Reason:        "Task spans multiple packages",
				SplitBy:       "package",
				ProposedTasks: proposed,
			})
		}
	}

	// Split tests from implementation (risk-isolation)
	testFiles := analysis.FileCategories["test"]
	sourceFiles := analysis.FileCategories["source"]
	if len(testFiles) > 0 && len(sourceFiles) > 0 {
		suggestions = append(suggestions, SplitSuggestion{
			Reason:  "Separating tests from implementation reduces risk",
			SplitBy: "risk-isolation",
			ProposedTasks: []ProposedTask{
				{
					Title:         task.Title + " - Implementation",
					Files:         sourceFiles,
					EstComplexity: estimateComplexity(len(sourceFiles)),
				},
				{
					Title:         task.Title + " - Tests",
					Files:         testFiles,
					EstComplexity: ultraplan.ComplexityLow,
				},
			},
		})
	}

	return suggestions
}

// estimateComplexity estimates task complexity from file count.
func estimateComplexity(fileCount int) ultraplan.TaskComplexity {
	switch {
	case fileCount > 5:
		return ultraplan.ComplexityHigh
	case fileCount > 2:
		return ultraplan.ComplexityMedium
	default:
		return ultraplan.ComplexityLow
	}
}

// calculateParallelismScore measures how well the plan utilizes parallelism.
func (a *Analyzer) calculateParallelismScore(spec *ultraplan.PlanSpec) int {
	if spec == nil || len(spec.Tasks) == 0 {
		return 0
	}

	taskCount := len(spec.Tasks)
	groupCount := len(spec.ExecutionOrder)

	if groupCount == 0 || taskCount == 0 {
		return 0
	}

	// Score = 100 * (tasks - groups + 1) / tasks
	// When groups == 1, score = 100
	// When groups == tasks, score approaches 0
	score := 100 * (taskCount - groupCount + 1) / taskCount
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// calculateAverageGroupSize returns the mean tasks per execution group.
func (a *Analyzer) calculateAverageGroupSize(spec *ultraplan.PlanSpec) float64 {
	if spec == nil || len(spec.ExecutionOrder) == 0 {
		return 0
	}

	total := 0
	for _, group := range spec.ExecutionOrder {
		total += len(group)
	}

	return float64(total) / float64(len(spec.ExecutionOrder))
}

// findBottleneckGroups identifies groups with only 1 task (sequential bottlenecks).
func (a *Analyzer) findBottleneckGroups(spec *ultraplan.PlanSpec) []int {
	var bottlenecks []int
	for i, group := range spec.ExecutionOrder {
		if len(group) == 1 {
			bottlenecks = append(bottlenecks, i)
		}
	}
	return bottlenecks
}

// findFileConflictClusters groups tasks by shared files.
func (a *Analyzer) findFileConflictClusters(spec *ultraplan.PlanSpec) []FileConflictCluster {
	// Map file -> tasks that modify it
	fileToTasks := make(map[string][]string)
	for _, task := range spec.Tasks {
		for _, file := range task.Files {
			fileToTasks[file] = append(fileToTasks[file], task.ID)
		}
	}

	// Build task group membership for conflict severity
	taskGroup := make(map[string]int)
	for groupIdx, group := range spec.ExecutionOrder {
		for _, taskID := range group {
			taskGroup[taskID] = groupIdx
		}
	}

	// Create clusters for files with multiple tasks
	var clusters []FileConflictCluster
	seenClusters := make(map[string]bool) // Dedupe by sorted task IDs

	for file, taskIDs := range fileToTasks {
		if len(taskIDs) <= 1 {
			continue
		}

		// Check if tasks are in the same group
		inSameGroup := false
		groups := make(map[int]bool)
		for _, tid := range taskIDs {
			groups[taskGroup[tid]] = true
		}
		if len(groups) < len(taskIDs) {
			inSameGroup = true
		}

		// Determine severity
		severity := "low"
		if inSameGroup {
			severity = "high"
		} else if len(taskIDs) > 2 {
			severity = "medium"
		}

		// Create cluster key for deduplication
		sortedIDs := make([]string, len(taskIDs))
		copy(sortedIDs, taskIDs)
		sort.Strings(sortedIDs)
		key := strings.Join(sortedIDs, ",")

		if seenClusters[key] {
			// Add file to existing cluster
			for i := range clusters {
				match := true
				if len(clusters[i].TaskIDs) != len(sortedIDs) {
					match = false
				} else {
					for j, id := range clusters[i].TaskIDs {
						if id != sortedIDs[j] {
							match = false
							break
						}
					}
				}
				if match {
					clusters[i].Files = append(clusters[i].Files, file)
					break
				}
			}
		} else {
			seenClusters[key] = true
			clusters = append(clusters, FileConflictCluster{
				Files:       []string{file},
				TaskIDs:     sortedIDs,
				InSameGroup: inSameGroup,
				Severity:    severity,
			})
		}
	}

	return clusters
}

// findPackageHotspots identifies packages modified by multiple tasks.
func (a *Analyzer) findPackageHotspots(spec *ultraplan.PlanSpec) []PackageHotspot {
	// Map package -> tasks and files
	type pkgInfo struct {
		tasks map[string]bool
		files map[string]bool
	}
	packageStats := make(map[string]*pkgInfo)

	for _, task := range spec.Tasks {
		for _, file := range task.Files {
			pkg := filepath.Dir(file)
			if pkg == "." || pkg == "" {
				continue
			}
			if packageStats[pkg] == nil {
				packageStats[pkg] = &pkgInfo{
					tasks: make(map[string]bool),
					files: make(map[string]bool),
				}
			}
			packageStats[pkg].tasks[task.ID] = true
			packageStats[pkg].files[file] = true
		}
	}

	var hotspots []PackageHotspot
	for pkg, info := range packageStats {
		if len(info.tasks) > 1 { // Only include if multiple tasks
			taskIDs := make([]string, 0, len(info.tasks))
			for id := range info.tasks {
				taskIDs = append(taskIDs, id)
			}
			sort.Strings(taskIDs)

			hotspots = append(hotspots, PackageHotspot{
				Package:   pkg,
				TaskCount: len(info.tasks),
				TaskIDs:   taskIDs,
				FileCount: len(info.files),
			})
		}
	}

	// Sort by task count descending
	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].TaskCount > hotspots[j].TaskCount
	})

	return hotspots
}

// optimizeExecutionOrder returns an optimized execution order based on blocking scores.
func (a *Analyzer) optimizeExecutionOrder(spec *ultraplan.PlanSpec, analyses map[string]*TaskAnalysis) [][]string {
	if spec == nil || len(spec.ExecutionOrder) == 0 {
		return nil
	}

	optimized := make([][]string, len(spec.ExecutionOrder))
	for i, group := range spec.ExecutionOrder {
		// Copy the group
		groupCopy := make([]string, len(group))
		copy(groupCopy, group)

		// Sort by blocking score (descending) - higher blocking score runs first
		sort.Slice(groupCopy, func(j, k int) bool {
			scoreJ := 0
			scoreK := 0
			if ta, ok := analyses[groupCopy[j]]; ok {
				scoreJ = ta.BlockingScore
			}
			if ta, ok := analyses[groupCopy[k]]; ok {
				scoreK = ta.BlockingScore
			}
			return scoreJ > scoreK
		})

		optimized[i] = groupCopy
	}

	return optimized
}

// performTransitiveReduction removes redundant dependencies from the plan.
// If A depends on B and B depends on C, then A→C is redundant.
// Returns optimized dependencies map and list of removed edges.
func (a *Analyzer) performTransitiveReduction(plan *ultraplan.PlanSpec) (map[string][]string, []TransitiveEdge) {
	if plan == nil || len(plan.Tasks) == 0 {
		return nil, nil
	}

	// Build adjacency list
	adj := make(map[string]map[string]bool)
	for _, task := range plan.Tasks {
		adj[task.ID] = make(map[string]bool)
		for _, dep := range task.DependsOn {
			adj[task.ID][dep] = true
		}
	}

	// Compute transitive closure using Floyd-Warshall
	closure := make(map[string]map[string]bool)
	taskIDs := make([]string, 0, len(plan.Tasks))
	for _, task := range plan.Tasks {
		taskIDs = append(taskIDs, task.ID)
		closure[task.ID] = make(map[string]bool)
		for dep := range adj[task.ID] {
			closure[task.ID][dep] = true
		}
	}

	for _, k := range taskIDs {
		for _, i := range taskIDs {
			for _, j := range taskIDs {
				if closure[i][k] && closure[k][j] {
					closure[i][j] = true
				}
			}
		}
	}

	// Find redundant edges
	var removed []TransitiveEdge
	for taskID, deps := range adj {
		for dep := range deps {
			// Check if this edge is redundant (reachable through other paths)
			for otherDep := range deps {
				if otherDep != dep && closure[otherDep][dep] {
					// dep is reachable from otherDep, so taskID→dep is redundant
					removed = append(removed, TransitiveEdge{
						From:   taskID,
						To:     dep,
						Reason: "Transitively implied via " + otherDep,
					})
					delete(adj[taskID], dep)
					break
				}
			}
		}
	}

	// Build optimized dependencies map
	optimized := make(map[string][]string)
	for taskID, deps := range adj {
		for dep := range deps {
			optimized[taskID] = append(optimized[taskID], dep)
		}
		// Sort for deterministic output
		sort.Strings(optimized[taskID])
	}

	return optimized, removed
}
