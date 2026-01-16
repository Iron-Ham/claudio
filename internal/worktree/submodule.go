package worktree

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SubmoduleInfo contains information about a git submodule.
type SubmoduleInfo struct {
	Name   string // The submodule name from .gitmodules
	Path   string // The relative path to the submodule
	URL    string // The submodule URL
	Branch string // Branch to track (empty if not specified)
}

// SubmoduleStatus represents the status of a submodule.
type SubmoduleStatus int

const (
	// SubmoduleUpToDate indicates the submodule is at the recorded commit.
	SubmoduleUpToDate SubmoduleStatus = iota
	// SubmoduleNotInitialized indicates the submodule needs to be initialized.
	SubmoduleNotInitialized
	// SubmoduleDifferentCommit indicates the submodule is at a different commit.
	SubmoduleDifferentCommit
	// SubmoduleMergeConflict indicates the submodule has merge conflicts.
	SubmoduleMergeConflict
)

func (s SubmoduleStatus) String() string {
	switch s {
	case SubmoduleUpToDate:
		return "up-to-date"
	case SubmoduleNotInitialized:
		return "not-initialized"
	case SubmoduleDifferentCommit:
		return "different-commit"
	case SubmoduleMergeConflict:
		return "merge-conflict"
	default:
		return "unknown"
	}
}

// SubmoduleStatusInfo contains status information about a submodule.
type SubmoduleStatusInfo struct {
	Path   string          // Path to the submodule relative to worktree root
	Commit string          // Current commit SHA (may be abbreviated)
	Status SubmoduleStatus // Current status indicator
}

// SubmoduleError represents an error during submodule operations.
type SubmoduleError struct {
	Operation string // The operation that failed (e.g., "init", "update")
	Output    string // Command output
	Err       error  // Underlying error
}

func (e *SubmoduleError) Error() string {
	return "submodule " + e.Operation + " failed: " + e.Err.Error() + "\n" + e.Output
}

func (e *SubmoduleError) Unwrap() error {
	return e.Err
}

// HasSubmodules checks if the repository has any git submodules configured.
// It checks for the presence of a .gitmodules file in the repository root.
func (m *Manager) HasSubmodules() bool {
	gitmodulesPath := filepath.Join(m.repoDir, ".gitmodules")
	info, err := os.Stat(gitmodulesPath)
	if err != nil {
		return false
	}
	// File must exist, be a regular file, and have content (not be empty)
	return info.Mode().IsRegular() && info.Size() > 0
}

// GetSubmodules returns a list of all submodules defined in the repository.
// Returns an empty slice if no submodules exist.
func (m *Manager) GetSubmodules() ([]SubmoduleInfo, error) {
	gitmodulesPath := filepath.Join(m.repoDir, ".gitmodules")
	if _, err := os.Stat(gitmodulesPath); os.IsNotExist(err) {
		return []SubmoduleInfo{}, nil
	}

	file, err := os.Open(gitmodulesPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return parseGitmodules(file)
}

// GetSubmodulePaths returns just the paths of all submodules.
// This is useful for filtering out submodule directories during file operations.
func (m *Manager) GetSubmodulePaths() ([]string, error) {
	submodules, err := m.GetSubmodules()
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(submodules))
	for _, sm := range submodules {
		if sm.Path != "" {
			paths = append(paths, sm.Path)
		}
	}
	return paths, nil
}

// InitSubmodules initializes and updates git submodules in the specified worktree path.
// This should be called after creating a worktree in a repository that has submodules.
// If the repository has no submodules, this is a no-op and returns nil.
//
// The function runs: git -c protocol.file.allow=always submodule update --init --recursive
// The protocol.file.allow=always flag is needed for git 2.38.1+ which disallows file transport
// by default for security, but we need it for local submodule references.
func (m *Manager) InitSubmodules(worktreePath string) error {
	if !m.HasSubmodules() {
		return nil
	}

	// Use -c protocol.file.allow=always to support local file:// submodule URLs
	// which are blocked by default in newer git versions for security
	args := []string{"-c", "protocol.file.allow=always", "submodule", "update", "--init", "--recursive"}
	cmd := exec.Command("git", args...)
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if m.logger != nil {
		m.logger.Debug("git submodule command", "args", args, "output", truncateOutput(string(output), 500))
	}

	if err != nil {
		// Check if this is a critical error or just a warning
		if isSubmoduleCriticalError(string(output)) {
			if m.logger != nil {
				m.logger.Error("git submodule command failed", "args", args, "error", err, "output", string(output))
			}
			return &SubmoduleError{
				Operation: "init",
				Output:    string(output),
				Err:       err,
			}
		}
		// Non-critical error - log warning but don't fail
		if m.logger != nil {
			m.logger.Warn("submodule initialization had issues",
				"path", worktreePath,
				"output", truncateOutput(string(output), 500))
		}
		return nil
	}

	if m.logger != nil {
		m.logger.Info("submodules initialized", "path", worktreePath)
	}

	return nil
}

// GetSubmoduleStatus returns the status of submodules in the given path.
// Returns nil if the repository has no submodules.
func (m *Manager) GetSubmoduleStatus(worktreePath string) ([]SubmoduleStatusInfo, error) {
	if !m.HasSubmodules() {
		return nil, nil
	}

	args := []string{"submodule", "status", "--recursive"}
	cmd := exec.Command("git", args...)
	cmd.Dir = worktreePath

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseSubmoduleStatus(string(output)), nil
}

// IsSubmodulePath checks if a given path (relative to repo root) is inside a submodule.
// This is useful for file watching to skip files inside submodules.
func (m *Manager) IsSubmodulePath(relativePath string) bool {
	paths, err := m.GetSubmodulePaths()
	if err != nil {
		return false
	}

	// Normalize path separators
	relativePath = filepath.ToSlash(relativePath)

	for _, smPath := range paths {
		smPath = filepath.ToSlash(smPath)
		// Check if relativePath starts with or equals the submodule path
		if relativePath == smPath || strings.HasPrefix(relativePath, smPath+"/") {
			return true
		}
	}
	return false
}

// IsSubmoduleDir checks if a directory is a git submodule by looking for
// a .git file (not directory) that points to the parent repo's .git/modules.
// In worktrees, submodule .git entries point to the main repo's modules.
//
// This function is safe to call on any path - it returns false if the
// path doesn't exist, can't be accessed, or isn't a submodule.
func IsSubmoduleDir(path string) bool {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		// Either path doesn't exist or we can't access it - either way, not a submodule
		return false
	}

	// A submodule has a .git file (not directory) that contains a gitdir reference
	if info.Mode().IsRegular() {
		content, err := os.ReadFile(gitPath)
		if err != nil {
			// Can't read the file - assume not a submodule
			return false
		}
		// Submodule .git files contain "gitdir: <path>"
		return strings.HasPrefix(string(content), "gitdir:")
	}

	return false
}

// parseGitmodules parses a .gitmodules file and returns submodule information.
func parseGitmodules(file *os.File) ([]SubmoduleInfo, error) {
	var submodules []SubmoduleInfo
	var current *SubmoduleInfo

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for section header [submodule "name"]
		if strings.HasPrefix(line, "[submodule ") {
			// Save previous submodule if exists and has a path
			if current != nil && current.Path != "" {
				submodules = append(submodules, *current)
			}

			// Parse the submodule name
			name := strings.TrimPrefix(line, "[submodule ")
			name = strings.TrimSuffix(name, "]")
			name = strings.Trim(name, "\"")

			current = &SubmoduleInfo{Name: name}
			continue
		}

		// Parse key = value pairs
		if current == nil {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "path":
			current.Path = value
		case "url":
			current.URL = value
		case "branch":
			current.Branch = value
		}
	}

	// Don't forget the last submodule
	if current != nil && current.Path != "" {
		submodules = append(submodules, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return submodules, nil
}

// parseSubmoduleStatus parses the output of 'git submodule status'.
func parseSubmoduleStatus(output string) []SubmoduleStatusInfo {
	var submodules []SubmoduleStatusInfo

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var status SubmoduleStatus
		switch line[0] {
		case ' ':
			status = SubmoduleUpToDate
		case '-':
			status = SubmoduleNotInitialized
		case '+':
			status = SubmoduleDifferentCommit
		case 'U':
			status = SubmoduleMergeConflict
		default:
			status = SubmoduleUpToDate // Default assumption
		}

		// Remove the status prefix and parse commit + path
		// Format: " <commit> <path> (<branch>)" or "-<commit> <path>"
		rest := strings.TrimLeft(line, " -+U")
		parts := strings.Fields(rest)
		if len(parts) >= 2 {
			submodules = append(submodules, SubmoduleStatusInfo{
				Commit: parts[0],
				Path:   parts[1],
				Status: status,
			})
		}
	}

	return submodules
}

// isSubmoduleCriticalError checks if the submodule output indicates a critical error
// that should fail worktree creation, vs a warning that can be safely ignored.
func isSubmoduleCriticalError(output string) bool {
	criticalPatterns := []string{
		"fatal:",                       // Git fatal errors
		"permission denied",            // Access issues
		"could not read from remote",   // Network/auth failures for required submodules
		"repository not found",         // Missing remote repository
		"unable to access",             // Access failures
		"authentication failed",        // Auth failures
		"host key verification failed", // SSH issues
		"no submodule mapping found",   // Corrupted .gitmodules
	}

	lowerOutput := strings.ToLower(output)
	for _, pattern := range criticalPatterns {
		if strings.Contains(lowerOutput, pattern) {
			return true
		}
	}

	// Check for clone failures specifically (they contain both "clone" and "failed")
	if strings.Contains(lowerOutput, "clone") && strings.Contains(lowerOutput, "failed") {
		return true
	}

	return false
}
