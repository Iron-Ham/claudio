package cmd

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
	"github.com/Iron-Ham/claudio/internal/tui"
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
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVar(&forceNew, "new", false, "Force start a new session")
	startCmd.Flags().StringVar(&attachSession, "session", "", "Attach to a specific session by ID")
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
		return attachToSession(cwd, attachSession, cfg)
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
	if len(unlockedSessions) > 0 && !forceNew {
		action, selectedID, err := promptMultiSessionAction(unlockedSessions)
		if err != nil {
			return err
		}

		switch action {
		case "attach":
			return attachToSession(cwd, selectedID, cfg)
		case "new":
			// Continue to create new session below
		case "list":
			return runSessionsList(cmd, args)
		case "quit":
			return nil
		}
	}

	// Create a new session
	return startNewSession(cwd, sessionName, cfg)
}

// attachToSession attaches to an existing session by ID
func attachToSession(cwd, sessionID string, cfg *config.Config) error {
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

	// Try to reconnect to running tmux sessions
	var reconnected []string
	for _, inst := range sess.Instances {
		mgr := orch.GetInstanceManager(inst.ID)
		if mgr == nil {
			continue
		}
		if mgr.TmuxSessionExists() {
			if err := mgr.Reconnect(); err == nil {
				inst.Status = orchestrator.StatusWorking
				inst.PID = mgr.PID()
				reconnected = append(reconnected, inst.ID)
			}
		}
	}

	if len(reconnected) > 0 {
		fmt.Printf("Reconnected to %d running instance(s)\n", len(reconnected))
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
	return attachToSession(cwd, sessionID, cfg)
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
func promptMultiSessionAction(sessions []*session.Info) (action string, selectedID string, err error) {
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
		fmt.Printf("  [%d] %s - %s (%d instances)%s\n", i+1, s.ID[:8], name, s.InstanceCount, lockStatus)
	}

	fmt.Println()
	fmt.Println("  [n] Create new session")
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

// checkStaleResourcesWarning checks for stale resources and prints a warning if found
func checkStaleResourcesWarning(baseDir string) {
	worktreesDir := filepath.Join(baseDir, ".claudio", "worktrees")

	var staleCount int

	// Count stale worktrees (directories in worktrees/ with no active session)
	entries, err := os.ReadDir(worktreesDir)
	if err == nil {
		staleCount += len(entries)
	}

	// Count stale branches
	cmd := exec.Command("git", "-C", baseDir, "branch", "--list", "claudio/*")
	if output, err := cmd.Output(); err == nil {
		branches := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, b := range branches {
			if strings.TrimSpace(b) != "" {
				staleCount++
			}
		}
	}

	// Count orphaned tmux sessions
	tmuxCmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
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
