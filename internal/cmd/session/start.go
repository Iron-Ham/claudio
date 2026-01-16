// Package session provides CLI commands for managing Claudio sessions.
// This includes starting, stopping, listing, and cleaning up sessions.
package session

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	orchsession "github.com/Iron-Ham/claudio/internal/orchestrator/session"
	"github.com/Iron-Ham/claudio/internal/session"
	"github.com/Iron-Ham/claudio/internal/tmux"
	"github.com/Iron-Ham/claudio/internal/tui"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var startCmd = &cobra.Command{
	Use:   "start [session-name]",
	Short: "Start a new Claudio session",
	Long: `Start a new Claudio session with an optional name.
This launches the TUI dashboard where you can add and manage Claude instances.

If other sessions exist, you will be prompted to attach to one or create a new session.
Multiple Claudio sessions can run simultaneously in different terminal windows.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStart,
}

var (
	forceNew      bool
	attachSession string // --session flag to attach to specific session
)

func init() {
	startCmd.Flags().BoolVar(&forceNew, "new", false, "Force start a new session")
	startCmd.Flags().StringVar(&attachSession, "session", "", "Attach to a specific session by ID")
}

// RegisterStartCmd registers the start command with the given parent command.
func RegisterStartCmd(parent *cobra.Command) {
	parent.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	sessionName := ""
	if len(args) > 0 {
		sessionName = args[0]
	}

	// Check for stale resources if configured
	if viper.GetBool("cleanup.warn_on_stale") {
		checkStaleResourcesWarning(cwd)
	}

	cfg := config.Get()

	// Check for legacy session format and prompt for migration
	legacyFile := filepath.Join(cwd, ".claudio", "session.json")
	if _, err := os.Stat(legacyFile); err == nil {
		fmt.Println("\nLegacy session format detected.")
		fmt.Println("Claudio now supports multiple simultaneous sessions.")
		fmt.Println("Your existing session will be migrated to the new format.")
		fmt.Println()

		return migrateAndStartLegacySession(cwd, sessionName, cfg)
	}

	// If --session flag is set, attach to that specific session
	if attachSession != "" {
		return AttachToSession(cwd, attachSession, cfg)
	}

	// List available sessions
	sessions, err := session.ListSessions(cwd)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	// Filter to unlocked sessions only
	var unlockedSessions []*session.Info
	for _, s := range sessions {
		if !s.IsLocked {
			unlockedSessions = append(unlockedSessions, s)
		}
	}

	// If there are unlocked sessions and we're not forcing new, prompt user
sessionLoop:
	for len(unlockedSessions) > 0 && !forceNew {
		action, selectedID, err := promptMultiSessionAction(cwd, unlockedSessions)
		if err != nil {
			return err
		}

		switch action {
		case "attach":
			return AttachToSession(cwd, selectedID, cfg)
		case "new":
			// Exit loop to create new session below
			break sessionLoop
		case "list":
			return RunSessionsList(cmd, args)
		case "quit":
			return nil
		case "refresh":
			// Re-list sessions after cleanup/shutdown
			sessions, err = session.ListSessions(cwd)
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}
			unlockedSessions = nil
			for _, s := range sessions {
				if !s.IsLocked {
					unlockedSessions = append(unlockedSessions, s)
				}
			}
			// Continue loop to re-prompt
		}
	}

	// Create a new session
	return startNewSession(cwd, sessionName, cfg)
}

// AttachToSession attaches to an existing session by ID.
// This is exported so other packages (like sessions command) can use it.
func AttachToSession(cwd, sessionID string, cfg *config.Config) error {
	// Check if session exists
	if !session.SessionExists(cwd, sessionID) {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Create logger if enabled
	sessionDir := session.GetSessionDir(cwd, sessionID)
	logger := CreateLogger(sessionDir, cfg)
	defer func() { _ = logger.Close() }()

	// Create orchestrator with the session ID
	orch, err := orchestrator.NewWithSession(cwd, sessionID, cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Set the logger on the orchestrator
	orch.SetLogger(logger)

	// Log startup
	logger.Info("claudio started", "session_id", sessionID, "mode", "attach")

	// Load and lock the session
	sess, err := orch.LoadSessionWithLock()
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	// Check for interrupted session that needs recovery
	needsRecovery := sess.NeedsRecovery()
	if needsRecovery {
		logger.Info("detected interrupted session, attempting recovery",
			"session_id", sessionID,
			"recovery_state", sess.RecoveryState,
		)
		fmt.Println("\nSession was interrupted - attempting recovery...")
		sess.MarkInstancesInterrupted()
	}

	// Try to reconnect to running tmux sessions or resume interrupted ones
	var reconnected []string
	var resumed []string
	for _, inst := range sess.Instances {
		mgr := orch.GetInstanceManager(inst.ID)
		if mgr == nil {
			continue
		}

		if mgr.TmuxSessionExists() {
			// Tmux session still exists - reconnect to it
			if err := mgr.Reconnect(); err != nil {
				logger.Warn("failed to reconnect to tmux session",
					"instance_id", inst.ID,
					"error", err.Error(),
				)
			} else {
				inst.Status = orchestrator.StatusWorking
				inst.PID = mgr.PID()
				reconnected = append(reconnected, inst.ID)
			}
		} else if needsRecovery && inst.ClaudeSessionID != "" &&
			(inst.Status == orchestrator.StatusWorking || inst.Status == orchestrator.StatusWaitingInput) {
			// Tmux session gone but we have Claude session ID - try to resume
			if err := orch.ResumeInstance(inst); err == nil {
				resumed = append(resumed, inst.ID)
				logger.Info("resumed interrupted instance",
					"instance_id", inst.ID,
					"claude_session_id", inst.ClaudeSessionID,
				)
			} else {
				logger.Warn("failed to resume instance, marking as paused",
					"instance_id", inst.ID,
					"error", err.Error(),
				)
				inst.Status = orchestrator.StatusPaused
			}
		}
	}

	if len(reconnected) > 0 {
		fmt.Printf("Reconnected to %d running instance(s)\n", len(reconnected))
	}
	if len(resumed) > 0 {
		fmt.Printf("Resumed %d interrupted instance(s)\n", len(resumed))
		sess.MarkRecovered()
	}

	// Check if this is an ultraplan session - if so, resume it
	if sess.UltraPlan != nil {
		return resumeUltraplanSession(orch, sess, logger)
	}

	// Check for active tripleshot sessions and restore them
	if len(sess.TripleShots) > 0 {
		return launchTUIWithTripleshots(cwd, orch, sess, logger)
	}

	return launchTUI(cwd, orch, sess, logger)
}

// startNewSession creates and starts a new session
func startNewSession(cwd, sessionName string, cfg *config.Config) error {
	// Generate a new session ID
	sessionID := orchsession.GenerateID()

	// Create logger if enabled - we need session dir which requires session ID
	sessionDir := session.GetSessionDir(cwd, sessionID)
	logger := CreateLogger(sessionDir, cfg)
	defer func() { _ = logger.Close() }()

	// Create orchestrator with the new session ID
	orch, err := orchestrator.NewWithSession(cwd, sessionID, cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Set the logger on the orchestrator
	orch.SetLogger(logger)

	// Start the new session
	sess, err := orch.StartSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	// Log startup
	logger.Info("claudio started", "session_id", sessionID, "mode", "new")

	fmt.Printf("Started new session: %s\n", sessionID)

	return launchTUI(cwd, orch, sess, logger)
}

// migrateAndStartLegacySession migrates a legacy session to the new format and starts it
func migrateAndStartLegacySession(cwd, sessionName string, cfg *config.Config) error {
	// First, load the legacy session to get its ID
	legacyOrch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	legacySession, err := legacyOrch.LoadSession()
	if err != nil {
		return fmt.Errorf("failed to load legacy session: %w", err)
	}

	// Use the existing session ID or generate a new one
	sessionID := legacySession.ID
	if sessionID == "" {
		sessionID = orchsession.GenerateID()
	}

	// Create the new session directory
	sessionDir := session.GetSessionDir(cwd, sessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Move the session file
	legacyFile := filepath.Join(cwd, ".claudio", "session.json")
	newFile := filepath.Join(sessionDir, "session.json")
	if err := os.Rename(legacyFile, newFile); err != nil {
		return fmt.Errorf("failed to migrate session file: %w", err)
	}

	// Move the context file if it exists
	legacyContext := filepath.Join(cwd, ".claudio", "context.md")
	if _, err := os.Stat(legacyContext); err == nil {
		newContext := filepath.Join(sessionDir, "context.md")
		_ = os.Rename(legacyContext, newContext)
	}

	fmt.Printf("Migrated session to: %s\n", sessionID)
	fmt.Println("Note: Existing tmux sessions use legacy naming and may need to be restarted.")
	fmt.Println()

	// Now start with the migrated session
	return AttachToSession(cwd, sessionID, cfg)
}

// launchTUI sets up terminal dimensions and launches the TUI
func launchTUI(cwd string, orch *orchestrator.Orchestrator, sess *orchestrator.Session, logger *logging.Logger) error {
	// Get terminal dimensions and set them on the orchestrator before launching TUI
	if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
		if contentWidth > 0 && contentHeight > 0 {
			orch.SetDisplayDimensions(contentWidth, contentHeight)
		}
	}

	// Launch TUI
	app := tui.New(orch, sess, logger.WithSession(sess.ID))
	if err := app.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// launchTUIWithTripleshots restores tripleshot sessions and launches the TUI.
// This is called when a session has active TripleShots that need to be restored.
func launchTUIWithTripleshots(cwd string, orch *orchestrator.Orchestrator, sess *orchestrator.Session, logger *logging.Logger) error {
	// Get terminal dimensions and set them on the orchestrator before launching TUI
	if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
		if contentWidth > 0 && contentHeight > 0 {
			orch.SetDisplayDimensions(contentWidth, contentHeight)
		}
	}

	// Restore tripleshot coordinators from persisted sessions
	var coordinators []*orchestrator.TripleShotCoordinator
	for _, tripleSession := range sess.TripleShots {
		// Skip completed or failed tripleshots
		if tripleSession.Phase == orchestrator.PhaseTripleShotComplete ||
			tripleSession.Phase == orchestrator.PhaseTripleShotFailed {
			continue
		}

		coordinator := orchestrator.NewTripleShotCoordinator(orch, sess, tripleSession, logger)
		coordinators = append(coordinators, coordinator)
		logger.Info("restored tripleshot session",
			"tripleshot_id", tripleSession.ID,
			"group_id", tripleSession.GroupID,
			"phase", string(tripleSession.Phase),
		)
	}

	// If we have active coordinators, launch with all of them restored
	var app *tui.App
	if len(coordinators) > 0 {
		fmt.Printf("Resuming %d active tripleshot session(s)\n", len(coordinators))
		app = tui.NewWithTripleShots(orch, sess, coordinators, logger.WithSession(sess.ID))
	} else {
		// All tripleshots completed, just launch normal TUI
		app = tui.New(orch, sess, logger.WithSession(sess.ID))
	}

	if err := app.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// resumeUltraplanSession resumes an interrupted ultraplan session.
// It detects the current phase and automatically continues execution from where it left off.
// This function uses the shared ultraplan initialization helpers to ensure consistency
// with newly created ultraplan sessions.
func resumeUltraplanSession(orch *orchestrator.Orchestrator, sess *orchestrator.Session, logger *logging.Logger) error {
	ultraSession := sess.UltraPlan

	// Log the resume attempt
	logger.Info("resuming ultraplan session",
		"session_id", sess.ID,
		"phase", string(ultraSession.Phase),
		"current_group", ultraSession.CurrentGroup,
	)

	// Print resume info to console
	groupInfo := ""
	if ultraSession.Plan != nil && len(ultraSession.Plan.ExecutionOrder) > 0 {
		groupInfo = fmt.Sprintf(", group: %d/%d", ultraSession.CurrentGroup+1, len(ultraSession.Plan.ExecutionOrder))
	}
	fmt.Printf("Resuming ultraplan session (phase: %s%s)\n", ultraSession.Phase, groupInfo)

	// Ensure the ultraplan group exists using the shared initialization helper.
	// This is important for resumed sessions where the group might not exist
	// (e.g., sessions created before group support was added, or if the group
	// was somehow lost). Creating the group BEFORE phase handling ensures that
	// any instances created during resume (e.g., restarting planning) are
	// properly added to the group for sidebar display.
	if ultraSession.GroupID == "" {
		ultraplan.CreateAndLinkUltraPlanGroup(sess, ultraSession, ultraSession.Config.MultiPass)
		logger.Info("created ultraplan group for resumed session",
			"group_id", ultraSession.GroupID,
			"multi_pass", ultraSession.Config.MultiPass,
		)
	}

	// Create coordinator from the loaded session state
	coordinator := orchestrator.NewCoordinator(orch, sess, ultraSession, logger)

	// Resume based on current phase
	switch ultraSession.Phase {
	case orchestrator.PhasePlanning:
		// Check if planning coordinator instance exists and is still running
		if ultraSession.CoordinatorID != "" && orch.GetInstance(ultraSession.CoordinatorID) != nil {
			fmt.Println("Planning in progress...")
		} else {
			fmt.Println("Restarting planning...")
			if err := coordinator.RunPlanning(); err != nil {
				return fmt.Errorf("failed to restart planning: %w", err)
			}
		}

	case orchestrator.PhasePlanSelection:
		fmt.Println("Plan selection in progress...")

	case orchestrator.PhaseRefresh:
		fmt.Println("Plan ready for review...")

	case orchestrator.PhaseExecuting:
		fmt.Println("Resuming execution...")
		if err := coordinator.StartExecution(); err != nil {
			return fmt.Errorf("failed to resume execution: %w", err)
		}

	case orchestrator.PhaseSynthesis:
		if ultraSession.SynthesisAwaitingApproval {
			fmt.Println("Synthesis complete, awaiting approval...")
		} else if ultraSession.SynthesisID != "" && orch.GetInstance(ultraSession.SynthesisID) != nil {
			fmt.Println("Synthesis in progress...")
		} else {
			fmt.Println("Restarting synthesis...")
			if err := coordinator.RunSynthesis(); err != nil {
				return fmt.Errorf("failed to restart synthesis: %w", err)
			}
		}

	case orchestrator.PhaseRevision:
		fmt.Println("Revision phase...")

	case orchestrator.PhaseConsolidating:
		fmt.Println("Consolidation in progress...")
		// Restart consolidation if the instance is gone
		if ultraSession.ConsolidationID != "" && orch.GetInstance(ultraSession.ConsolidationID) == nil {
			if err := coordinator.StartConsolidation(); err != nil {
				return fmt.Errorf("failed to restart consolidation: %w", err)
			}
		}

	case orchestrator.PhaseComplete:
		fmt.Println("Session already completed. Use [R] to re-trigger a group.")

	case orchestrator.PhaseFailed:
		fmt.Printf("Session previously failed: %s\n", ultraSession.Error)
		fmt.Println("Use [R] to re-trigger a group or [r] to retry failed tasks.")

	default:
		logger.Error("unknown ultraplan phase encountered",
			"phase", string(ultraSession.Phase),
			"session_id", sess.ID,
		)
		return fmt.Errorf("unknown ultraplan phase: %s", ultraSession.Phase)
	}

	// Get terminal dimensions and set them on the orchestrator
	if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err != nil {
		logger.Info("terminal size detection failed, using defaults",
			"error", err.Error(),
		)
	} else {
		contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
		if contentWidth > 0 && contentHeight > 0 {
			orch.SetDisplayDimensions(contentWidth, contentHeight)
		}
	}

	// Launch TUI in ultraplan mode
	app := tui.NewWithUltraPlan(orch, sess, coordinator, logger.WithSession(sess.ID))
	return app.Run()
}

// CreateLogger creates a logger if logging is enabled in config.
// Returns a NopLogger if logging is disabled or if creation fails.
// This function uses NewLoggerWithRotation to respect MaxSizeMB and MaxBackups config.
func CreateLogger(sessionDir string, cfg *config.Config) *logging.Logger {
	// Check if logging is enabled
	if !cfg.Logging.Enabled {
		return logging.NopLogger()
	}

	// Build rotation config from logging config
	rotationConfig := logging.RotationConfig{
		MaxSizeMB:  cfg.Logging.MaxSizeMB,
		MaxBackups: cfg.Logging.MaxBackups,
		Compress:   false, // Not exposed in config yet
	}

	// Create the logger with rotation support
	logger, err := logging.NewLoggerWithRotation(sessionDir, cfg.Logging.Level, rotationConfig)
	if err != nil {
		// Log creation failure shouldn't prevent the application from starting
		fmt.Fprintf(os.Stderr, "Warning: failed to create logger: %v\n", err)
		return logging.NopLogger()
	}

	return logger
}

// promptMultiSessionAction prompts the user to choose what to do when sessions exist
func promptMultiSessionAction(baseDir string, sessions []*session.Info) (action string, selectedID string, err error) {
	// Count empty sessions
	var emptyCount int
	for _, s := range sessions {
		if s.InstanceCount == 0 && !s.IsLocked {
			emptyCount++
		}
	}

	fmt.Println("\nExisting sessions found:")
	fmt.Println()

	for i, s := range sessions {
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		lockStatus := ""
		if s.IsLocked {
			lockStatus = " [LOCKED]"
		}
		fmt.Printf("  [%d] %s - %s (%d instances)%s\n", i+1, session.TruncateID(s.ID, 8), name, s.InstanceCount, lockStatus)
	}

	fmt.Println()
	fmt.Println("  [n] Create new session")
	if emptyCount > 0 {
		fmt.Printf("  [c] Clean up %d empty session(s)\n", emptyCount)
	}
	fmt.Println("  [s] Shutdown all sessions")
	fmt.Println("  [l] List all sessions with details")
	fmt.Println("  [q] Quit")
	fmt.Println()
	fmt.Print("Enter number to attach or action [1/n/l/q]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "n", "new":
		return "new", "", nil
	case "c", "clean":
		if emptyCount > 0 {
			cleanEmptySessions(baseDir)
			return "refresh", "", nil
		}
		fmt.Println("No empty sessions to clean")
		return "refresh", "", nil
	case "s", "shutdown":
		shutdownAllSessions()
		return "refresh", "", nil
	case "l", "list":
		return "list", "", nil
	case "q", "quit":
		return "quit", "", nil
	case "", "1":
		// Default to first session
		if len(sessions) > 0 {
			return "attach", sessions[0].ID, nil
		}
		return "new", "", nil
	default:
		// Try to parse as number
		var idx int
		if _, err := fmt.Sscanf(input, "%d", &idx); err == nil {
			if idx >= 1 && idx <= len(sessions) {
				return "attach", sessions[idx-1].ID, nil
			}
		}
		// Check if it's a session ID prefix
		for _, s := range sessions {
			if strings.HasPrefix(s.ID, input) {
				return "attach", s.ID, nil
			}
		}
		fmt.Printf("Unknown option '%s', creating new session\n", input)
		return "new", "", nil
	}
}

// cleanEmptySessions removes all empty sessions (0 instances)
func cleanEmptySessions(baseDir string) {
	emptySessions, err := session.FindEmptySessions(baseDir)
	if err != nil {
		fmt.Printf("Error finding empty sessions: %v\n", err)
		return
	}

	if len(emptySessions) == 0 {
		fmt.Println("No empty sessions to clean")
		return
	}

	var removed int
	for _, s := range emptySessions {
		if err := session.RemoveSession(baseDir, s.ID); err != nil {
			fmt.Printf("Warning: failed to remove session %s: %v\n", session.TruncateID(s.ID, 8), err)
			continue
		}
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("Removed empty session: %s - %s\n", session.TruncateID(s.ID, 8), name)
		removed++
	}
	fmt.Printf("Cleaned %d empty session(s)\n\n", removed)
}

// shutdownAllSessions kills all claudio-* tmux sessions
func shutdownAllSessions() {
	cmd := tmux.Command("list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// Check if it's just "no server running" which is expected
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "no server running") {
				fmt.Println("No tmux sessions running")
				return
			}
		}
		fmt.Printf("Error listing tmux sessions: %v\n", err)
		return
	}

	var killed int
	tmuxSessions := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, sess := range tmuxSessions {
		sess = strings.TrimSpace(sess)
		if sess == "" || !strings.HasPrefix(sess, "claudio-") {
			continue
		}

		killCmd := tmux.Command("kill-session", "-t", sess)
		if err := killCmd.Run(); err != nil {
			fmt.Printf("Warning: failed to kill tmux session %s: %v\n", sess, err)
			continue
		}
		fmt.Printf("Killed tmux session: %s\n", sess)
		killed++
	}
	if killed > 0 {
		fmt.Printf("Shutdown %d tmux session(s)\n\n", killed)
	} else {
		fmt.Println("No claudio tmux sessions running")
	}
}

// checkStaleResourcesWarning checks for stale resources and prints a warning if found
func checkStaleResourcesWarning(baseDir string) {
	cfg := config.Get()
	worktreesDir := cfg.Paths.ResolveWorktreeDir(baseDir)
	branchPrefix := cfg.Branch.Prefix
	if branchPrefix == "" {
		branchPrefix = "claudio"
	}

	var staleCount int

	// Count stale worktrees (directories in worktrees/ with no active session)
	entries, err := os.ReadDir(worktreesDir)
	if err == nil {
		staleCount += len(entries)
	}

	// Count stale branches
	cmd := exec.Command("git", "-C", baseDir, "branch", "--list", branchPrefix+"/*")
	if output, err := cmd.Output(); err == nil {
		branches := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, b := range branches {
			if strings.TrimSpace(b) != "" {
				staleCount++
			}
		}
	}

	// Count orphaned tmux sessions
	tmuxCmd := tmux.Command("list-sessions", "-F", "#{session_name}")
	if output, err := tmuxCmd.Output(); err == nil {
		sessions := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, s := range sessions {
			if strings.HasPrefix(strings.TrimSpace(s), "claudio-") {
				staleCount++
			}
		}
	}

	if staleCount > 0 {
		fmt.Printf("Warning: Found %d potentially stale resources. Run 'claudio cleanup --dry-run' to review.\n\n", staleCount)
	}
}
