package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/pr"
	"github.com/Iron-Ham/claudio/internal/worktree"
	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use:   "pr [instance-id]",
	Short: "Create a pull request for a Claudio instance",
	Long: `Create a GitHub pull request for a Claudio instance using Claude to generate
a meaningful title and description based on the task, code changes, and commit history.

If no instance ID is provided and there's only one instance, it will be used automatically.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPR,
}

var (
	prDraft     bool
	prNoPush    bool
	prNoAI      bool
	prNoRebase  bool
	prTitle     string
	prBody      string
	prReviewers []string
	prLabels    []string
	prCloses    []string
)

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.Flags().BoolVarP(&prDraft, "draft", "d", false, "Create as a draft PR")
	prCmd.Flags().BoolVar(&prNoPush, "no-push", false, "Don't push the branch before creating the PR")
	prCmd.Flags().BoolVar(&prNoAI, "no-ai", false, "Skip AI generation, use simple defaults")
	prCmd.Flags().BoolVar(&prNoRebase, "no-rebase", false, "Skip rebasing on main before creating PR")
	prCmd.Flags().StringVarP(&prTitle, "title", "t", "", "Override the PR title")
	prCmd.Flags().StringVarP(&prBody, "body", "b", "", "Override the PR body")
	prCmd.Flags().StringSliceVarP(&prReviewers, "reviewer", "r", nil, "Add reviewers (can be specified multiple times)")
	prCmd.Flags().StringSliceVarP(&prLabels, "label", "l", nil, "Add labels (can be specified multiple times)")
	prCmd.Flags().StringSliceVar(&prCloses, "closes", nil, "Link issues to close (e.g., --closes 42)")
}

func runPR(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Load config for defaults
	cfg := config.Get()

	// Apply config defaults, then allow flags to override
	// Note: flags override config when explicitly set
	useDraft := cfg.PR.Draft
	if cmd.Flags().Changed("draft") {
		useDraft = prDraft
	}
	useRebase := cfg.PR.AutoRebase
	if cmd.Flags().Changed("no-rebase") {
		useRebase = !prNoRebase
	}
	useAI := cfg.PR.UseAI
	if cmd.Flags().Changed("no-ai") {
		useAI = !prNoAI
	}

	// Create orchestrator and load session
	orch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	session, err := orch.LoadSession()
	if err != nil {
		return fmt.Errorf("no active session found: %w", err)
	}

	// Find the instance
	var inst *orchestrator.Instance
	if len(args) > 0 {
		inst = session.GetInstance(args[0])
		if inst == nil {
			return fmt.Errorf("instance %s not found", args[0])
		}
	} else {
		// If no instance specified, use the only one or error
		if len(session.Instances) == 0 {
			return fmt.Errorf("no instances in session")
		}
		if len(session.Instances) > 1 {
			return fmt.Errorf("multiple instances exist, please specify an instance ID")
		}
		inst = session.Instances[0]
	}

	fmt.Printf("Creating PR for instance %s...\n", inst.ID)
	fmt.Printf("Task: %s\n", inst.Task)
	fmt.Printf("Branch: %s\n\n", inst.Branch)

	// Create worktree manager
	wt, err := worktree.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create worktree manager: %w", err)
	}

	// Check for uncommitted changes
	hasChanges, err := wt.HasUncommittedChanges(inst.WorktreePath)
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}
	if hasChanges {
		return fmt.Errorf("instance has uncommitted changes. Please commit or stash them first")
	}

	// Get changed files
	changedFiles, err := wt.GetChangedFiles(inst.WorktreePath)
	if err != nil {
		return fmt.Errorf("failed to get changed files: %w", err)
	}
	if len(changedFiles) == 0 {
		return fmt.Errorf("no changes to create PR for")
	}

	fmt.Printf("Changed files: %d\n", len(changedFiles))

	// Check for rebase conflicts and rebase if needed
	if useRebase {
		behindCount, err := wt.GetBehindCount(inst.WorktreePath)
		if err != nil {
			fmt.Printf("Warning: could not check if branch is behind main: %v\n", err)
		} else if behindCount > 0 {
			fmt.Printf("Branch is %d commit(s) behind main, checking for conflicts...\n", behindCount)

			hasConflicts, err := wt.HasRebaseConflicts(inst.WorktreePath)
			if err != nil {
				fmt.Printf("Warning: could not check for conflicts: %v\n", err)
			} else if hasConflicts {
				return fmt.Errorf("rebasing would cause conflicts. Please resolve manually:\n  cd %s\n  git rebase origin/main\n  # resolve conflicts\n  git rebase --continue", inst.WorktreePath)
			}

			fmt.Println("Rebasing on main...")
			if err := wt.RebaseOnMain(inst.WorktreePath); err != nil {
				return fmt.Errorf("failed to rebase: %w", err)
			}
			fmt.Println("Rebase successful!")
		} else {
			fmt.Println("Branch is up to date with main.")
		}
	}

	// Push branch if needed
	if !prNoPush {
		fmt.Println("Pushing branch to remote...")
		// Force push after rebase (with lease for safety)
		forceNeeded := useRebase
		if err := wt.Push(inst.WorktreePath, forceNeeded); err != nil {
			return fmt.Errorf("failed to push branch: %w", err)
		}
	}

	// Extract issue references from task description
	linkedIssue := pr.ExtractIssueReference(inst.Task)

	// Collect issues from --closes flag and task description
	var closesIssues []string
	closesIssues = append(closesIssues, prCloses...)
	if linkedIssue != "" && !containsIssue(closesIssues, linkedIssue) {
		closesIssues = append(closesIssues, linkedIssue)
	}

	// Generate or use provided PR content
	var title, body string
	var aiSummary string

	if prTitle != "" && prBody != "" {
		// User provided both title and body
		title = prTitle
		body = prBody
	} else if !useAI {
		// Simple defaults without AI
		title = fmt.Sprintf("feat: %s", truncateString(inst.Task, 60))
		body = fmt.Sprintf("## Task\n%s\n\n## Changed Files\n", inst.Task)
		for _, f := range changedFiles {
			body += fmt.Sprintf("- %s\n", f)
		}
	} else {
		// Use Claude to generate PR content
		fmt.Println("Generating PR content with Claude...")

		diff, err := wt.GetDiffAgainstMain(inst.WorktreePath)
		if err != nil {
			fmt.Printf("Warning: failed to get diff: %v\n", err)
			diff = ""
		}

		commitLog, err := wt.GetCommitLog(inst.WorktreePath)
		if err != nil {
			fmt.Printf("Warning: failed to get commit log: %v\n", err)
			commitLog = ""
		}

		gen := pr.New()
		content, err := gen.Generate(pr.Context{
			Task:         inst.Task,
			Branch:       inst.Branch,
			Diff:         diff,
			CommitLog:    commitLog,
			ChangedFiles: changedFiles,
			InstanceID:   inst.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to generate PR content: %w", err)
		}

		title = content.Title
		body = content.Body
		aiSummary = content.Body

		// Allow overrides
		if prTitle != "" {
			title = prTitle
		}
		if prBody != "" {
			body = prBody
		}
	}

	// Apply custom template if configured
	if cfg.PR.Template != "" && prBody == "" {
		commitLog, _ := wt.GetCommitLog(inst.WorktreePath)
		templateData := pr.TemplateData{
			AISummary:    aiSummary,
			Task:         inst.Task,
			Branch:       inst.Branch,
			ChangedFiles: changedFiles,
			CommitLog:    commitLog,
			LinkedIssue:  linkedIssue,
			InstanceID:   inst.ID,
		}

		renderedBody, err := pr.RenderTemplate(cfg.PR.Template, templateData)
		if err != nil {
			fmt.Printf("Warning: failed to render PR template: %v\n", err)
		} else {
			body = renderedBody
		}
	}

	// Append closes clause if we have linked issues
	if len(closesIssues) > 0 {
		closesClause := pr.FormatClosesClause(closesIssues)
		if !strings.Contains(body, closesClause) {
			body = body + "\n\n" + closesClause
		}
	}

	// Resolve reviewers: combine config defaults, path-based rules, and CLI flags
	reviewers := pr.ResolveReviewers(changedFiles, cfg.PR.Reviewers.Default, cfg.PR.Reviewers.ByPath)
	// Add CLI-specified reviewers
	for _, r := range prReviewers {
		r = strings.TrimPrefix(r, "@")
		if !containsString(reviewers, r) {
			reviewers = append(reviewers, r)
		}
	}

	// Resolve labels: combine config defaults and CLI flags
	labels := append([]string{}, cfg.PR.Labels...)
	for _, l := range prLabels {
		if !containsString(labels, l) {
			labels = append(labels, l)
		}
	}

	fmt.Printf("\nTitle: %s\n", title)
	fmt.Println("\nBody:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println(body)
	fmt.Println(strings.Repeat("-", 40))

	if len(reviewers) > 0 {
		fmt.Printf("\nReviewers: %s\n", strings.Join(reviewers, ", "))
	}
	if len(labels) > 0 {
		fmt.Printf("Labels: %s\n", strings.Join(labels, ", "))
	}

	// Create the PR using the unified Create function
	fmt.Println("\nCreating pull request...")
	prURL, err := pr.Create(pr.PROptions{
		Title:     title,
		Body:      body,
		Branch:    inst.Branch,
		Draft:     useDraft,
		Reviewers: reviewers,
		Labels:    labels,
	})
	if err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}

	fmt.Printf("\nPull request created: %s\n", prURL)
	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func containsIssue(issues []string, issue string) bool {
	issue = strings.TrimPrefix(issue, "#")
	for _, i := range issues {
		if strings.TrimPrefix(i, "#") == issue {
			return true
		}
	}
	return false
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
