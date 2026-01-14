// Package decomposition provides enhanced task decomposition capabilities for Ultra-Plan.
//
// This package implements sophisticated analysis techniques to improve how tasks
// are broken down and scheduled for parallel execution:
//
// # Code Structure Analysis
//
// The analyzer examines the codebase structure to understand relationships:
//   - Package dependencies and import graphs
//   - File coupling (files that change together)
//   - Type and function references across files
//   - Test file associations
//
// # Risk-Based Prioritization
//
// Tasks are scored for risk to inform execution strategy:
//   - High-risk tasks (core modules, many dependents) run early/sequentially
//   - Low-risk tasks (leaf modules, isolated changes) can run in parallel
//   - Risk factors: file count, complexity, centrality, test coverage
//
// # Critical Path Analysis
//
// The critical path through the dependency graph is optimized:
//   - Identifies the longest chain of dependent tasks
//   - Suggests parallelization opportunities to shorten total time
//   - Recommends task splits when critical path is dominated by one task
//
// # Dependency Inference
//
// Automatically infers task dependencies from code analysis:
//   - Tasks modifying the same package gain soft dependencies
//   - Tasks touching shared interfaces gain ordering constraints
//   - Import chain analysis suggests execution ordering
//
// Usage:
//
//	analyzer := decomposition.NewAnalyzer(repoPath)
//	analysis, err := analyzer.Analyze(plan)
//	enhancedPlan := analysis.Apply(plan)
package decomposition
