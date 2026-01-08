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
		return m, nil

	case tea.KeyMsg:
		// Clear messages on any key
		m.errorMsg = ""
		m.infoMsg = ""

		if m.editing {
			return m.handleEditingKeypress(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
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

		case "tab":
			// Move to next category
			m.categoryIndex++
			if m.categoryIndex >= len(m.categories) {
				m.categoryIndex = 0
			}
			m.itemIndex = 0

		case "shift+tab":
			// Move to previous category
			m.categoryIndex--
			if m.categoryIndex < 0 {
				m.categoryIndex = len(m.categories) - 1
			}
			m.itemIndex = 0

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
			viper.Set(item.Key, item.Options[m.selectIndex])
			m.saveConfig()
			m.editing = false
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

	// Categories and items
	for ci, cat := range m.categories {
		isActiveCategory := ci == m.categoryIndex

		// Category header
		catStyle := styles.Muted.Bold(true)
		if isActiveCategory {
			catStyle = styles.Primary.Bold(true)
		}
		b.WriteString(catStyle.Render(fmt.Sprintf("[ %s ]", cat.Name)))
		b.WriteString("\n")

		for ii, item := range cat.Items {
			isSelected := isActiveCategory && ii == m.itemIndex
			b.WriteString(m.renderItem(item, isSelected))
			b.WriteString("\n")
		}
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
		return helpStyle.Render(
			keyStyle.Render("enter") + " save  " +
				keyStyle.Render("esc") + " cancel",
		)
	}

	return helpStyle.Render(
		keyStyle.Render("j/k") + " navigate  " +
			keyStyle.Render("tab") + " next category  " +
			keyStyle.Render("enter/space") + " edit  " +
			keyStyle.Render("r") + " reset  " +
			keyStyle.Render("q") + " quit",
	)
}

func (m Model) currentItem() ConfigItem {
	return m.categories[m.categoryIndex].Items[m.itemIndex]
}

func (m Model) getCurrentValue() string {
	item := m.currentItem()
	switch item.Type {
	case "bool":
		return fmt.Sprintf("%v", viper.GetBool(item.Key))
	case "int":
		return fmt.Sprintf("%d", viper.GetInt(item.Key))
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
		viper.Set(item.Key, intVal)
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
	defaultValues := map[string]interface{}{
		"completion.default_action":             defaults.Completion.DefaultAction,
		"tui.auto_focus_on_input":               defaults.TUI.AutoFocusOnInput,
		"tui.max_output_lines":                  defaults.TUI.MaxOutputLines,
		"instance.output_buffer_size":           defaults.Instance.OutputBufferSize,
		"instance.capture_interval_ms":          defaults.Instance.CaptureIntervalMs,
		"instance.tmux_width":                   defaults.Instance.TmuxWidth,
		"instance.tmux_height":                  defaults.Instance.TmuxHeight,
		"instance.activity_timeout_minutes":     defaults.Instance.ActivityTimeoutMinutes,
		"instance.completion_timeout_minutes":   defaults.Instance.CompletionTimeoutMinutes,
		"instance.stale_detection":              defaults.Instance.StaleDetection,
		"pr.draft":                              defaults.PR.Draft,
		"pr.auto_rebase":                        defaults.PR.AutoRebase,
		"pr.use_ai":                             defaults.PR.UseAI,
		"pr.auto_pr_on_stop":                    defaults.PR.AutoPROnStop,
	}

	if defaultVal, ok := defaultValues[item.Key]; ok {
		viper.Set(item.Key, defaultVal)
		m.saveConfig()
		m.infoMsg = fmt.Sprintf("Reset %s to default", item.Label)
	}
}

// Run starts the interactive config UI
func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
