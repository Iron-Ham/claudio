package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
)

// ConfigItem represents a single configuration item
type ConfigItem struct {
	Key         string
	Label       string
	Description string
	Type        string   // "string", "bool", "int", "select"
	Options     []string // For select type
	Category    string
}

// Category represents a group of config items
type Category struct {
	Name  string
	Items []ConfigItem
}

// Model is the Bubbletea model for the interactive config UI
type Model struct {
	categories     []Category
	categoryIndex  int
	itemIndex      int
	width          int
	height         int
	scrollOffset   int // Line offset for scrolling
	editing        bool
	textInput      textinput.Model
	selectIndex    int // For select-type options
	errorMsg       string
	infoMsg        string
	quitting       bool
	configModified bool
}

// New creates a new config model
func New() Model {
	// Discover custom themes so they appear in the theme selector
	_, _ = styles.DiscoverCustomThemes()

	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	categories := []Category{
		{
			Name: "Completion",
			Items: []ConfigItem{
				{
					Key:         "completion.default_action",
					Label:       "Default Action",
					Description: "Action when an instance completes its task",
					Type:        "select",
					Options:     config.ValidCompletionActions(),
					Category:    "completion",
				},
			},
		},
		{
			Name: "TUI",
			Items: []ConfigItem{
				{
					Key:         "tui.theme",
					Label:       "Color Theme",
					Description: "Color theme for the TUI (changes apply immediately)",
					Type:        "select",
					Options:     styles.ValidThemes(),
					Category:    "tui",
				},
				{
					Key:         "tui.auto_focus_on_input",
					Label:       "Auto Focus on Input",
					Description: "Automatically focus new instances for input",
					Type:        "bool",
					Category:    "tui",
				},
				{
					Key:         "tui.max_output_lines",
					Label:       "Max Output Lines",
					Description: "Maximum lines of output to display per instance",
					Type:        "int",
					Category:    "tui",
				},
				{
					Key:         "tui.verbose_command_help",
					Label:       "Verbose Command Help",
					Description: "Show full command descriptions in command mode (disable for compact view)",
					Type:        "bool",
					Category:    "tui",
				},
				{
					Key:         "tui.sidebar_width",
					Label:       "Sidebar Width",
					Description: "Width of the sidebar panel in columns (20-60, default: 36)",
					Type:        "int",
					Category:    "tui",
				},
			},
		},
		{
			Name: "Instance",
			Items: []ConfigItem{
				{
					Key:         "instance.output_buffer_size",
					Label:       "Output Buffer Size",
					Description: "Output buffer size in bytes",
					Type:        "int",
					Category:    "instance",
				},
				{
					Key:         "instance.capture_interval_ms",
					Label:       "Capture Interval (ms)",
					Description: "How often to capture output from tmux",
					Type:        "int",
					Category:    "instance",
				},
				{
					Key:         "instance.tmux_width",
					Label:       "Tmux Width",
					Description: "Width of the tmux pane",
					Type:        "int",
					Category:    "instance",
				},
				{
					Key:         "instance.tmux_height",
					Label:       "Tmux Height",
					Description: "Height of the tmux pane",
					Type:        "int",
					Category:    "instance",
				},
				{
					Key:         "instance.activity_timeout_minutes",
					Label:       "Activity Timeout (min)",
					Description: "Minutes of no output before marking as stuck (0 = disabled)",
					Type:        "int",
					Category:    "instance",
				},
				{
					Key:         "instance.completion_timeout_minutes",
					Label:       "Max Runtime (min)",
					Description: "Maximum runtime in minutes before timeout (0 = disabled)",
					Type:        "int",
					Category:    "instance",
				},
				{
					Key:         "instance.stale_detection",
					Label:       "Stale Detection",
					Description: "Detect stuck loops producing identical output",
					Type:        "bool",
					Category:    "instance",
				},
				{
					Key:         "instance.tmux_history_limit",
					Label:       "Tmux History Limit",
					Description: "Number of lines of scrollback to keep in tmux",
					Type:        "int",
					Category:    "instance",
				},
			},
		},
		{
			Name: "Pull Request",
			Items: []ConfigItem{
				{
					Key:         "pr.draft",
					Label:       "Draft PRs",
					Description: "Create PRs as drafts by default",
					Type:        "bool",
					Category:    "pr",
				},
				{
					Key:         "pr.auto_rebase",
					Label:       "Auto Rebase",
					Description: "Rebase on main before creating PR",
					Type:        "bool",
					Category:    "pr",
				},
				{
					Key:         "pr.use_ai",
					Label:       "Use AI",
					Description: "Use Claude AI to generate PR content",
					Type:        "bool",
					Category:    "pr",
				},
				{
					Key:         "pr.auto_pr_on_stop",
					Label:       "Auto PR on Stop",
					Description: "Automatically commit, push, and create PR when stopping an instance with 'x'",
					Type:        "bool",
					Category:    "pr",
				},
				{
					Key:         "pr.labels",
					Label:       "Default Labels",
					Description: "Comma-separated list of labels to add to all PRs",
					Type:        "string",
					Category:    "pr",
				},
				{
					Key:         "pr.reviewers.default",
					Label:       "Default Reviewers",
					Description: "Comma-separated list of GitHub usernames to assign as reviewers",
					Type:        "string",
					Category:    "pr",
				},
			},
		},
		{
			Name: "Branch",
			Items: []ConfigItem{
				{
					Key:         "branch.prefix",
					Label:       "Branch Prefix",
					Description: "Prefix for auto-generated branch names (e.g., 'claudio', 'feature')",
					Type:        "string",
					Category:    "branch",
				},
				{
					Key:         "branch.include_id",
					Label:       "Include Instance ID",
					Description: "Include instance ID in branch names for uniqueness",
					Type:        "bool",
					Category:    "branch",
				},
			},
		},
		{
			Name: "Cleanup",
			Items: []ConfigItem{
				{
					Key:         "cleanup.warn_on_stale",
					Label:       "Warn on Stale",
					Description: "Show warning on start if stale resources exist",
					Type:        "bool",
					Category:    "cleanup",
				},
				{
					Key:         "cleanup.keep_remote_branches",
					Label:       "Keep Remote Branches",
					Description: "Prevent deletion of branches that exist on remote",
					Type:        "bool",
					Category:    "cleanup",
				},
			},
		},
		{
			Name: "Resources",
			Items: []ConfigItem{
				{
					Key:         "resources.cost_warning_threshold",
					Label:       "Cost Warning ($)",
					Description: "Trigger warning when session cost exceeds this amount (USD)",
					Type:        "float",
					Category:    "resources",
				},
				{
					Key:         "resources.cost_limit",
					Label:       "Cost Limit ($)",
					Description: "Pause instances when session cost exceeds this (0 = no limit)",
					Type:        "float",
					Category:    "resources",
				},
				{
					Key:         "resources.token_limit_per_instance",
					Label:       "Token Limit/Instance",
					Description: "Max tokens per instance (0 = no limit)",
					Type:        "int",
					Category:    "resources",
				},
				{
					Key:         "resources.show_metrics_in_sidebar",
					Label:       "Show Metrics",
					Description: "Show token/cost metrics in TUI sidebar",
					Type:        "bool",
					Category:    "resources",
				},
			},
		},
		{
			Name: "Ultraplan",
			Items: []ConfigItem{
				{
					Key:         "ultraplan.max_parallel",
					Label:       "Max Parallel Tasks",
					Description: "Maximum concurrent child sessions (0 = unlimited)",
					Type:        "int",
					Category:    "ultraplan",
				},
				{
					Key:         "ultraplan.multi_pass",
					Label:       "Multi-Pass Planning",
					Description: "Use multiple coordinators for more thorough plans",
					Type:        "bool",
					Category:    "ultraplan",
				},
				{
					Key:         "ultraplan.consolidation_mode",
					Label:       "Consolidation Mode",
					Description: "How to consolidate completed work: stacked or single PR",
					Type:        "select",
					Options:     []string{"stacked", "single"},
					Category:    "ultraplan",
				},
				{
					Key:         "ultraplan.create_draft_prs",
					Label:       "Create Draft PRs",
					Description: "Create PRs as drafts during consolidation",
					Type:        "bool",
					Category:    "ultraplan",
				},
				{
					Key:         "ultraplan.pr_labels",
					Label:       "PR Labels",
					Description: "Comma-separated labels to add to ultraplan PRs",
					Type:        "string",
					Category:    "ultraplan",
				},
				{
					Key:         "ultraplan.branch_prefix",
					Label:       "Branch Prefix",
					Description: "Override branch prefix for ultraplan (empty = use branch.prefix)",
					Type:        "string",
					Category:    "ultraplan",
				},
				{
					Key:         "ultraplan.max_task_retries",
					Label:       "Max Task Retries",
					Description: "Max retry attempts for tasks that produce no commits",
					Type:        "int",
					Category:    "ultraplan",
				},
				{
					Key:         "ultraplan.require_verified_commits",
					Label:       "Require Verified Commits",
					Description: "Require tasks to produce commits to be marked successful",
					Type:        "bool",
					Category:    "ultraplan",
				},
				{
					Key:         "ultraplan.notifications.enabled",
					Label:       "Notifications",
					Description: "Enable audio notifications when user input is needed",
					Type:        "bool",
					Category:    "ultraplan",
				},
				{
					Key:         "ultraplan.notifications.use_sound",
					Label:       "Use System Sound",
					Description: "Play macOS system sound in addition to terminal bell",
					Type:        "bool",
					Category:    "ultraplan",
				},
				{
					Key:         "ultraplan.notifications.sound_path",
					Label:       "Custom Sound Path",
					Description: "Path to custom sound file (macOS only, leave empty for default)",
					Type:        "string",
					Category:    "ultraplan",
				},
			},
		},
		{
			Name: "Plan",
			Items: []ConfigItem{
				{
					Key:         "plan.output_format",
					Label:       "Output Format",
					Description: "Default output format: json, issues, or both",
					Type:        "select",
					Options:     []string{"json", "issues", "both"},
					Category:    "plan",
				},
				{
					Key:         "plan.multi_pass",
					Label:       "Multi-Pass Planning",
					Description: "Use 3-strategy planning for more thorough plans",
					Type:        "bool",
					Category:    "plan",
				},
				{
					Key:         "plan.labels",
					Label:       "Issue Labels",
					Description: "Comma-separated labels to add to GitHub Issues",
					Type:        "string",
					Category:    "plan",
				},
				{
					Key:         "plan.output_file",
					Label:       "Output File",
					Description: "Default JSON output file path",
					Type:        "string",
					Category:    "plan",
				},
			},
		},
		{
			Name: "Adversarial",
			Items: []ConfigItem{
				{
					Key:         "adversarial.max_iterations",
					Label:       "Max Iterations",
					Description: "Maximum implement-review cycles (0 = unlimited)",
					Type:        "int",
					Category:    "adversarial",
				},
				{
					Key:         "adversarial.min_passing_score",
					Label:       "Min Passing Score",
					Description: "Minimum reviewer score for approval (1-10)",
					Type:        "int",
					Category:    "adversarial",
				},
			},
		},
		{
			Name: "Paths",
			Items: []ConfigItem{
				{
					Key:         "paths.worktree_dir",
					Label:       "Worktree Directory",
					Description: "Where git worktrees are created (empty = .claudio/worktrees, supports ~ and relative paths)",
					Type:        "string",
					Category:    "paths",
				},
				{
					Key:         "paths.sparse_checkout.enabled",
					Label:       "Sparse Checkout",
					Description: "Enable sparse checkout for partial worktrees (requires directories configured in config.yaml)",
					Type:        "bool",
					Category:    "paths",
				},
				{
					Key:         "paths.sparse_checkout.cone_mode",
					Label:       "Sparse Cone Mode",
					Description: "Use faster cone mode for sparse checkout (directory paths only, no wildcards)",
					Type:        "bool",
					Category:    "paths",
				},
				{
					Key:         "paths.sparse_checkout.directories",
					Label:       "Sparse Directories",
					Description: "Directories to include in sparse checkout (edit config.yaml to modify array)",
					Type:        "string",
					Category:    "paths",
				},
				{
					Key:         "paths.sparse_checkout.always_include",
					Label:       "Sparse Always Include",
					Description: "Directories always included in sparse checkout (edit config.yaml to modify array)",
					Type:        "string",
					Category:    "paths",
				},
			},
		},
		{
			Name: "Logging",
			Items: []ConfigItem{
				{
					Key:         "logging.enabled",
					Label:       "Enabled",
					Description: "Enable debug logging to file",
					Type:        "bool",
					Category:    "logging",
				},
				{
					Key:         "logging.level",
					Label:       "Log Level",
					Description: "Minimum log level to record",
					Type:        "select",
					Options:     []string{"debug", "info", "warn", "error"},
					Category:    "logging",
				},
				{
					Key:         "logging.max_size_mb",
					Label:       "Max Size (MB)",
					Description: "Maximum log file size before rotation",
					Type:        "int",
					Category:    "logging",
				},
				{
					Key:         "logging.max_backups",
					Label:       "Max Backups",
					Description: "Number of backup log files to keep",
					Type:        "int",
					Category:    "logging",
				},
			},
		},
		{
			Name: "Experimental",
			Items: []ConfigItem{
				{
					Key:         "experimental.intelligent_naming",
					Label:       "Intelligent Naming",
					Description: "Use Claude to generate short, descriptive instance names (requires ANTHROPIC_API_KEY)",
					Type:        "bool",
					Category:    "experimental",
				},
				{
					Key:         "experimental.terminal_support",
					Label:       "Terminal Support",
					Description: "Enable embedded terminal pane (:term, :t, :termdir commands)",
					Type:        "bool",
					Category:    "experimental",
				},
				{
					Key:         "experimental.inline_plan",
					Label:       "Inline MultiPlan",
					Description: "Enable :multiplan command for multi-pass planning with 3 planners + assessor",
					Type:        "bool",
					Category:    "experimental",
				},
				{
					Key:         "experimental.inline_ultraplan",
					Label:       "Inline UltraPlan",
					Description: "Enable :ultraplan command in standard TUI to start UltraPlan workflows",
					Type:        "bool",
					Category:    "experimental",
				},
				{
					Key:         "experimental.grouped_instance_view",
					Label:       "Grouped Instance View",
					Description: "Enable visual grouping of instances by execution group in the sidebar",
					Type:        "bool",
					Category:    "experimental",
				},
			},
		},
	}

	return Model{
		categories: categories,
		textInput:  ti,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureSelectionVisible(m.calculateAvailableLines())
		return m, nil

	case tea.KeyMsg:
		// Clear messages on any key
		m.errorMsg = ""
		m.infoMsg = ""

		if m.editing {
			return m.handleEditingKeypress(msg)
		}

		switch msg.String() {
		case "esc":
			if m.configModified {
				m.infoMsg = "Changes saved!"
			}
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			m.itemIndex--
			if m.itemIndex < 0 {
				// Move to previous category
				m.categoryIndex--
				if m.categoryIndex < 0 {
					m.categoryIndex = len(m.categories) - 1
				}
				m.itemIndex = len(m.categories[m.categoryIndex].Items) - 1
			}
			m.ensureSelectionVisible(m.calculateAvailableLines())

		case "down", "j":
			m.itemIndex++
			if m.itemIndex >= len(m.categories[m.categoryIndex].Items) {
				// Move to next category
				m.categoryIndex++
				if m.categoryIndex >= len(m.categories) {
					m.categoryIndex = 0
				}
				m.itemIndex = 0
			}
			m.ensureSelectionVisible(m.calculateAvailableLines())

		case "tab":
			// Move to next category
			m.categoryIndex++
			if m.categoryIndex >= len(m.categories) {
				m.categoryIndex = 0
			}
			m.itemIndex = 0
			m.ensureSelectionVisible(m.calculateAvailableLines())

		case "shift+tab":
			// Move to previous category
			m.categoryIndex--
			if m.categoryIndex < 0 {
				m.categoryIndex = len(m.categories) - 1
			}
			m.itemIndex = 0
			m.ensureSelectionVisible(m.calculateAvailableLines())

		case "ctrl+d", "pgdown":
			// Page down - move half a screen
			halfPage := m.calculateAvailableLines() / 2
			for i := 0; i < halfPage; i++ {
				m.itemIndex++
				if m.itemIndex >= len(m.categories[m.categoryIndex].Items) {
					m.categoryIndex++
					if m.categoryIndex >= len(m.categories) {
						// Stop at the last item
						m.categoryIndex = len(m.categories) - 1
						m.itemIndex = len(m.categories[m.categoryIndex].Items) - 1
						break
					}
					m.itemIndex = 0
				}
			}
			m.ensureSelectionVisible(m.calculateAvailableLines())

		case "ctrl+u", "pgup":
			// Page up - move half a screen
			halfPage := m.calculateAvailableLines() / 2
			for i := 0; i < halfPage; i++ {
				m.itemIndex--
				if m.itemIndex < 0 {
					m.categoryIndex--
					if m.categoryIndex < 0 {
						// Stop at the first item
						m.categoryIndex = 0
						m.itemIndex = 0
						break
					}
					m.itemIndex = len(m.categories[m.categoryIndex].Items) - 1
				}
			}
			m.ensureSelectionVisible(m.calculateAvailableLines())

		case "g":
			// Go to first item
			m.categoryIndex = 0
			m.itemIndex = 0
			m.ensureSelectionVisible(m.calculateAvailableLines())

		case "G":
			// Go to last item
			m.categoryIndex = len(m.categories) - 1
			m.itemIndex = len(m.categories[m.categoryIndex].Items) - 1
			m.ensureSelectionVisible(m.calculateAvailableLines())

		case "enter", " ":
			item := m.currentItem()
			switch item.Type {
			case "bool":
				// Toggle boolean directly
				current := viper.GetBool(item.Key)
				viper.Set(item.Key, !current)
				m.saveConfig()
			case "select":
				// Enter selection mode
				m.editing = true
				m.selectIndex = m.getCurrentSelectIndex()
			default:
				// Enter edit mode for int/string
				m.editing = true
				m.textInput.SetValue(m.getCurrentValue())
				m.textInput.Focus()
			}

		case "r":
			// Reset current item to default
			m.resetCurrentToDefault()
		}
	}

	return m, cmd
}

func (m *Model) handleEditingKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	item := m.currentItem()

	switch msg.String() {
	case "esc":
		m.editing = false
		m.textInput.SetValue("")
		return m, nil

	case "enter":
		if item.Type == "select" {
			// Apply selected option
			selectedValue := item.Options[m.selectIndex]
			viper.Set(item.Key, selectedValue)
			m.saveConfig()
			m.editing = false

			// Apply theme change immediately for live preview
			if item.Key == "tui.theme" {
				styles.SetActiveTheme(styles.ThemeName(selectedValue))
			}
		} else {
			// Validate and apply text input
			value := m.textInput.Value()
			if err := m.validateAndSet(item, value); err != nil {
				m.errorMsg = err.Error()
				return m, nil
			}
			m.saveConfig()
			m.editing = false
			m.textInput.SetValue("")
		}
		return m, nil

	case "up", "k":
		if item.Type == "select" {
			m.selectIndex--
			if m.selectIndex < 0 {
				m.selectIndex = len(item.Options) - 1
			}
		}
		return m, nil

	case "down", "j":
		if item.Type == "select" {
			m.selectIndex++
			if m.selectIndex >= len(item.Options) {
				m.selectIndex = 0
			}
		}
		return m, nil
	}

	// Handle text input
	if item.Type != "select" {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Header
	header := styles.Header.Width(m.width - 4).Render("Claudio Configuration")
	b.WriteString(header)
	b.WriteString("\n\n")

	// Config file path
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		configPath = config.ConfigFile() + " (not created)"
	}
	b.WriteString(styles.Muted.Render(fmt.Sprintf("Config file: %s", configPath)))
	b.WriteString("\n\n")

	availableLines := m.calculateAvailableLines()

	// Build all content lines
	var allLines []string
	for ci, cat := range m.categories {
		isActiveCategory := ci == m.categoryIndex

		// Category header
		catStyle := styles.Muted.Bold(true)
		if isActiveCategory {
			catStyle = styles.Primary.Bold(true)
		}
		allLines = append(allLines, catStyle.Render(fmt.Sprintf("[ %s ]", cat.Name)))

		for ii, item := range cat.Items {
			isSelected := isActiveCategory && ii == m.itemIndex
			allLines = append(allLines, m.renderItem(item, isSelected))
		}
		allLines = append(allLines, "") // Blank line after category
	}

	// Calculate scroll bounds
	totalLines := len(allLines)
	scrollOffset := m.scrollOffset

	// Clamp scroll offset
	maxScroll := totalLines - availableLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Show scroll up indicator
	hasMoreAbove := scrollOffset > 0
	if hasMoreAbove {
		b.WriteString(styles.Muted.Render(fmt.Sprintf("  ▲ %d more above", scrollOffset)))
		b.WriteString("\n")
	}

	// Render visible lines
	endLine := scrollOffset + availableLines
	if endLine > totalLines {
		endLine = totalLines
	}

	for i := scrollOffset; i < endLine; i++ {
		b.WriteString(allLines[i])
		b.WriteString("\n")
	}

	// Show scroll down indicator
	hasMoreBelow := endLine < totalLines
	if hasMoreBelow {
		remaining := totalLines - endLine
		b.WriteString(styles.Muted.Render(fmt.Sprintf("  ▼ %d more below", remaining)))
		b.WriteString("\n")
	}

	// Add padding if content is shorter than available space
	if !hasMoreAbove && !hasMoreBelow {
		// No scrolling needed, but add a blank line for consistency
		b.WriteString("\n")
	}

	// Edit overlay or description
	if m.editing {
		b.WriteString(m.renderEditOverlay())
	} else {
		// Show description for current item
		item := m.currentItem()
		b.WriteString(styles.Muted.Render(item.Description))
		b.WriteString("\n")
	}

	// Error/Info messages
	if m.errorMsg != "" {
		b.WriteString("\n")
		b.WriteString(styles.ErrorMsg.Render("Error: " + m.errorMsg))
	}
	if m.infoMsg != "" {
		b.WriteString("\n")
		b.WriteString(styles.SuccessMsg.Render(m.infoMsg))
	}

	// Help bar
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

func (m Model) renderItem(item ConfigItem, selected bool) string {
	value := m.getDisplayValue(item)

	// Build the line
	label := item.Label
	if len(label) > 25 {
		label = label[:22] + "..."
	}

	// Pad label to align values
	paddedLabel := fmt.Sprintf("%-25s", label)

	var line string
	if selected {
		cursor := styles.Secondary.Render(">")
		labelStyled := styles.Text.Bold(true).Render(paddedLabel)
		valueStyled := styles.Primary.Render(value)
		line = fmt.Sprintf("  %s %s  %s", cursor, labelStyled, valueStyled)
	} else {
		labelStyled := styles.Muted.Render(paddedLabel)
		valueStyled := styles.Text.Render(value)
		line = fmt.Sprintf("    %s  %s", labelStyled, valueStyled)
	}

	return line
}

func (m Model) renderEditOverlay() string {
	item := m.currentItem()
	var b strings.Builder

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.PrimaryColor).
		Padding(1, 2).
		Width(50)

	var content string
	if item.Type == "select" {
		content = fmt.Sprintf("Select %s:\n\n", item.Label)
		for i, opt := range item.Options {
			if i == m.selectIndex {
				content += styles.DropdownItemSelected.Render(fmt.Sprintf(" > %s ", opt)) + "\n"
			} else {
				content += styles.DropdownItem.Render(fmt.Sprintf("   %s ", opt)) + "\n"
			}
		}
		content += "\n" + styles.Muted.Render("j/k or arrows to select, enter to confirm, esc to cancel")
	} else {
		content = fmt.Sprintf("Edit %s:\n\n", item.Label)
		content += m.textInput.View()
		content += "\n\n" + styles.Muted.Render("enter to save, esc to cancel")
	}

	b.WriteString("\n")
	b.WriteString(borderStyle.Render(content))

	return b.String()
}

func (m Model) renderHelp() string {
	helpStyle := styles.HelpBar
	keyStyle := styles.HelpKey

	if m.editing {
		badge := styles.ModeBadgeInput.Render("EDITING")
		return helpStyle.Render(
			badge + "  " +
				keyStyle.Render("enter") + " save  " +
				keyStyle.Render("esc") + " cancel",
		)
	}

	badge := styles.ModeBadgeNormal.Render("CONFIG")
	return helpStyle.Render(
		badge + "  " +
			keyStyle.Render("j/k") + " navigate  " +
			keyStyle.Render("ctrl+d/u") + " page  " +
			keyStyle.Render("g/G") + " top/bottom  " +
			keyStyle.Render("tab") + " category  " +
			keyStyle.Render("enter") + " edit  " +
			keyStyle.Render("r") + " reset  " +
			keyStyle.Render("esc") + " quit",
	)
}

func (m Model) currentItem() ConfigItem {
	return m.categories[m.categoryIndex].Items[m.itemIndex]
}

// calculateAvailableLines returns the number of lines available for scrollable content
func (m Model) calculateAvailableLines() int {
	// Reserve lines for: header (2), config path (2), description (2), messages (2), help (2), scroll indicators (2)
	const reservedLines = 12
	availableLines := m.height - reservedLines
	if availableLines < 5 {
		availableLines = 5 // Minimum visible lines
	}
	return availableLines
}

// totalLines returns the total number of lines needed to render all categories and items
func (m Model) totalLines() int {
	lines := 0
	for _, cat := range m.categories {
		lines++ // Category header
		lines += len(cat.Items)
		lines++ // Blank line after category
	}
	return lines
}

// currentSelectionLine returns the line number (0-indexed) of the currently selected item
func (m Model) currentSelectionLine() int {
	line := 0
	for ci, cat := range m.categories {
		if ci == m.categoryIndex {
			line++ // Category header
			line += m.itemIndex
			return line
		}
		line++ // Category header
		line += len(cat.Items)
		line++ // Blank line after category
	}
	return line
}

// ensureSelectionVisible adjusts scrollOffset to keep the current selection visible
func (m *Model) ensureSelectionVisible(availableLines int) {
	if availableLines <= 0 {
		return
	}

	selectionLine := m.currentSelectionLine()

	// If selection is above viewport, scroll up
	if selectionLine < m.scrollOffset {
		m.scrollOffset = selectionLine
	}

	// If selection is below viewport, scroll down
	// We want the selection to be visible, so check if it's past the bottom
	if selectionLine >= m.scrollOffset+availableLines {
		m.scrollOffset = selectionLine - availableLines + 1
	}

	// Clamp scroll offset
	maxScroll := m.totalLines() - availableLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m Model) getCurrentValue() string {
	item := m.currentItem()
	switch item.Type {
	case "bool":
		return fmt.Sprintf("%v", viper.GetBool(item.Key))
	case "int":
		return fmt.Sprintf("%d", viper.GetInt(item.Key))
	case "float":
		return fmt.Sprintf("%.2f", viper.GetFloat64(item.Key))
	default:
		return viper.GetString(item.Key)
	}
}

func (m Model) getDisplayValue(item ConfigItem) string {
	switch item.Type {
	case "bool":
		if viper.GetBool(item.Key) {
			return "true"
		}
		return "false"
	case "int":
		return fmt.Sprintf("%d", viper.GetInt(item.Key))
	case "float":
		return fmt.Sprintf("%.2f", viper.GetFloat64(item.Key))
	default:
		return viper.GetString(item.Key)
	}
}

func (m Model) getCurrentSelectIndex() int {
	item := m.currentItem()
	current := viper.GetString(item.Key)
	for i, opt := range item.Options {
		if opt == current {
			return i
		}
	}
	return 0
}

func (m *Model) validateAndSet(item ConfigItem, value string) error {
	switch item.Type {
	case "int":
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("expected integer value")
		}
		if intVal < 0 {
			return fmt.Errorf("value must be non-negative")
		}
		// Item-specific validation for fields with special constraints
		if item.Key == "adversarial.min_passing_score" {
			if intVal < 1 || intVal > 10 {
				return fmt.Errorf("value must be between 1 and 10")
			}
		}
		viper.Set(item.Key, intVal)
	case "float":
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("expected decimal value")
		}
		if floatVal < 0 {
			return fmt.Errorf("value must be non-negative")
		}
		viper.Set(item.Key, floatVal)
	case "bool":
		if value != "true" && value != "false" {
			return fmt.Errorf("expected true or false")
		}
		viper.Set(item.Key, value == "true")
	case "select":
		valid := false
		for _, opt := range item.Options {
			if opt == value {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid option: %s", value)
		}
		viper.Set(item.Key, value)
	default:
		viper.Set(item.Key, value)
	}
	return nil
}

func (m *Model) saveConfig() {
	// Ensure config directory exists
	configDir := config.ConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		m.errorMsg = fmt.Sprintf("Failed to create config directory: %v", err)
		return
	}

	configFile := config.ConfigFile()
	if err := viper.WriteConfigAs(configFile); err != nil {
		m.errorMsg = fmt.Sprintf("Failed to save config: %v", err)
		return
	}

	m.infoMsg = "Saved!"
	m.configModified = true
}

func (m *Model) resetCurrentToDefault() {
	item := m.currentItem()
	defaults := config.Default()

	// Map of keys to default values
	defaultValues := map[string]any{
		// Completion
		"completion.default_action": defaults.Completion.DefaultAction,
		// TUI
		"tui.theme":                defaults.TUI.Theme,
		"tui.auto_focus_on_input":  defaults.TUI.AutoFocusOnInput,
		"tui.max_output_lines":     defaults.TUI.MaxOutputLines,
		"tui.verbose_command_help": defaults.TUI.VerboseCommandHelp,
		"tui.sidebar_width":        defaults.TUI.SidebarWidth,
		// Instance
		"instance.output_buffer_size":         defaults.Instance.OutputBufferSize,
		"instance.capture_interval_ms":        defaults.Instance.CaptureIntervalMs,
		"instance.tmux_width":                 defaults.Instance.TmuxWidth,
		"instance.tmux_height":                defaults.Instance.TmuxHeight,
		"instance.tmux_history_limit":         defaults.Instance.TmuxHistoryLimit,
		"instance.activity_timeout_minutes":   defaults.Instance.ActivityTimeoutMinutes,
		"instance.completion_timeout_minutes": defaults.Instance.CompletionTimeoutMinutes,
		"instance.stale_detection":            defaults.Instance.StaleDetection,
		// Pull Request
		"pr.draft":             defaults.PR.Draft,
		"pr.auto_rebase":       defaults.PR.AutoRebase,
		"pr.use_ai":            defaults.PR.UseAI,
		"pr.auto_pr_on_stop":   defaults.PR.AutoPROnStop,
		"pr.labels":            strings.Join(defaults.PR.Labels, ","),
		"pr.reviewers.default": strings.Join(defaults.PR.Reviewers.Default, ","),
		// Branch
		"branch.prefix":     defaults.Branch.Prefix,
		"branch.include_id": defaults.Branch.IncludeID,
		// Cleanup
		"cleanup.warn_on_stale":        defaults.Cleanup.WarnOnStale,
		"cleanup.keep_remote_branches": defaults.Cleanup.KeepRemoteBranches,
		// Resources
		"resources.cost_warning_threshold":   defaults.Resources.CostWarningThreshold,
		"resources.cost_limit":               defaults.Resources.CostLimit,
		"resources.token_limit_per_instance": defaults.Resources.TokenLimitPerInstance,
		"resources.show_metrics_in_sidebar":  defaults.Resources.ShowMetricsInSidebar,
		// Ultraplan
		"ultraplan.max_parallel":             defaults.Ultraplan.MaxParallel,
		"ultraplan.multi_pass":               defaults.Ultraplan.MultiPass,
		"ultraplan.consolidation_mode":       defaults.Ultraplan.ConsolidationMode,
		"ultraplan.create_draft_prs":         defaults.Ultraplan.CreateDraftPRs,
		"ultraplan.pr_labels":                strings.Join(defaults.Ultraplan.PRLabels, ","),
		"ultraplan.branch_prefix":            defaults.Ultraplan.BranchPrefix,
		"ultraplan.max_task_retries":         defaults.Ultraplan.MaxTaskRetries,
		"ultraplan.require_verified_commits": defaults.Ultraplan.RequireVerifiedCommits,
		"ultraplan.notifications.enabled":    defaults.Ultraplan.Notifications.Enabled,
		"ultraplan.notifications.use_sound":  defaults.Ultraplan.Notifications.UseSound,
		"ultraplan.notifications.sound_path": defaults.Ultraplan.Notifications.SoundPath,
		// Plan
		"plan.output_format": defaults.Plan.OutputFormat,
		"plan.multi_pass":    defaults.Plan.MultiPass,
		"plan.labels":        strings.Join(defaults.Plan.Labels, ","),
		"plan.output_file":   defaults.Plan.OutputFile,
		// Adversarial
		"adversarial.max_iterations":    defaults.Adversarial.MaxIterations,
		"adversarial.min_passing_score": defaults.Adversarial.MinPassingScore,
		// Paths
		"paths.worktree_dir":                   defaults.Paths.WorktreeDir,
		"paths.sparse_checkout.enabled":        defaults.Paths.SparseCheckout.Enabled,
		"paths.sparse_checkout.cone_mode":      defaults.Paths.SparseCheckout.ConeMode,
		"paths.sparse_checkout.directories":    strings.Join(defaults.Paths.SparseCheckout.Directories, ","),
		"paths.sparse_checkout.always_include": strings.Join(defaults.Paths.SparseCheckout.AlwaysInclude, ","),
		// Logging
		"logging.enabled":     defaults.Logging.Enabled,
		"logging.level":       defaults.Logging.Level,
		"logging.max_size_mb": defaults.Logging.MaxSizeMB,
		"logging.max_backups": defaults.Logging.MaxBackups,
		// Experimental
		"experimental.intelligent_naming":    defaults.Experimental.IntelligentNaming,
		"experimental.terminal_support":      defaults.Experimental.TerminalSupport,
		"experimental.inline_plan":           defaults.Experimental.InlinePlan,
		"experimental.inline_ultraplan":      defaults.Experimental.InlineUltraPlan,
		"experimental.grouped_instance_view": defaults.Experimental.GroupedInstanceView,
	}

	if defaultVal, ok := defaultValues[item.Key]; ok {
		viper.Set(item.Key, defaultVal)
		m.saveConfig()
		m.infoMsg = fmt.Sprintf("Reset %s to default", item.Label)

		// Apply theme change immediately when resetting theme
		if item.Key == "tui.theme" {
			if themeName, ok := defaultVal.(string); ok {
				styles.SetActiveTheme(styles.ThemeName(themeName))
			}
		}
	}
}

// Run starts the interactive config UI
func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
