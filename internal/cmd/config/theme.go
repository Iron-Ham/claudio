package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/spf13/cobra"
)

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage color themes",
	Long: `Manage color themes for the Claudio TUI.

Claudio supports both built-in themes and custom user-defined themes.
Custom themes are stored in ~/.config/claudio/themes/ as YAML files.

Use 'theme list' to see all available themes.
Use 'theme export' to create a template for custom themes.
Use 'theme info' to view details about a specific theme.`,
}

var themeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available themes",
	RunE:  runThemeList,
}

var themeExportCmd = &cobra.Command{
	Use:   "export <theme-name> [output-file]",
	Short: "Export a theme to YAML",
	Long: `Export a theme to YAML format for customization or sharing.

If no output file is specified, the YAML is printed to stdout.
This is useful for creating a starting point for custom themes.

Examples:
  claudio config theme export default              # Print default theme to stdout
  claudio config theme export dracula my-theme.yaml  # Save dracula theme to file
  claudio config theme export default > custom.yaml  # Redirect to file`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runThemeExport,
}

var themeInfoCmd = &cobra.Command{
	Use:   "info <theme-name>",
	Short: "Show information about a theme",
	Args:  cobra.ExactArgs(1),
	RunE:  runThemeInfo,
}

var themePathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the custom themes directory path",
	RunE:  runThemePath,
}

var themeCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new custom theme from the default template",
	Long: `Create a new custom theme file in your themes directory.

This creates a new YAML file based on the default theme that you can customize.
The theme will be available after restarting Claudio or running 'claudio' again.

Example:
  claudio config theme create solarized
  # Creates ~/.config/claudio/themes/solarized.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runThemeCreate,
}

func init() {
	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeExportCmd)
	themeCmd.AddCommand(themeInfoCmd)
	themeCmd.AddCommand(themePathCmd)
	themeCmd.AddCommand(themeCreateCmd)
	configCmd.AddCommand(themeCmd)
}

func runThemeList(cmd *cobra.Command, args []string) error {
	// Discover custom themes and report any load errors
	_, loadErrs := styles.DiscoverCustomThemes()
	if len(loadErrs) > 0 {
		fmt.Fprintln(os.Stderr, "Warning: Some themes failed to load:")
		for _, err := range loadErrs {
			fmt.Fprintf(os.Stderr, "  - %v\n", err)
		}
		fmt.Fprintln(os.Stderr)
	}

	fmt.Println("Available themes:")
	fmt.Println()

	// Built-in themes
	fmt.Println("Built-in themes:")
	for _, name := range styles.BuiltinThemes() {
		fmt.Printf("  - %s\n", name)
	}

	// Custom themes
	customNames := styles.CustomThemeNames()
	if len(customNames) > 0 {
		fmt.Println()
		fmt.Println("Custom themes:")
		sort.Strings(customNames)
		for _, name := range customNames {
			theme := styles.GetCustomTheme(styles.ThemeName(name))
			if theme != nil {
				if theme.Author != "" {
					fmt.Printf("  - %s (by %s)\n", name, theme.Author)
				} else {
					fmt.Printf("  - %s\n", name)
				}
			}
		}
	}

	fmt.Println()
	fmt.Printf("Custom themes directory: %s\n", styles.ThemesDir())

	return nil
}

func runThemeExport(cmd *cobra.Command, args []string) error {
	themeName := args[0]

	// Discover custom themes first
	_, loadErrs := styles.DiscoverCustomThemes()

	if !styles.IsValidTheme(themeName) {
		// Check if there was a load error for this specific theme
		for _, err := range loadErrs {
			errStr := err.Error()
			if strings.HasPrefix(errStr, themeName+".yaml:") || strings.HasPrefix(errStr, themeName+".yml:") || strings.HasPrefix(errStr, themeName+":") {
				return fmt.Errorf("theme '%s' exists but failed to load: %v\n\nFix the errors in your theme file and try again", themeName, err)
			}
		}
		return fmt.Errorf("unknown theme: %s\n\nRun 'claudio config theme list' to see available themes.\nCustom themes should be placed in: %s", themeName, styles.ThemesDir())
	}

	data, err := styles.ExportTheme(styles.ThemeName(themeName))
	if err != nil {
		return fmt.Errorf("exporting theme: %w", err)
	}

	// If output file specified, write to file
	if len(args) > 1 {
		outputPath := args[1]
		if err := os.WriteFile(outputPath, data, 0o644); err != nil {
			return fmt.Errorf("writing to %s: %w", outputPath, err)
		}
		fmt.Printf("Theme exported to: %s\n", outputPath)
		return nil
	}

	// Otherwise print to stdout
	fmt.Println(string(data))
	return nil
}

func runThemeInfo(cmd *cobra.Command, args []string) error {
	themeName := args[0]

	// Discover custom themes first
	_, loadErrs := styles.DiscoverCustomThemes()

	if !styles.IsValidTheme(themeName) {
		// Check if there was a load error for this specific theme
		for _, err := range loadErrs {
			errStr := err.Error()
			if strings.HasPrefix(errStr, themeName+".yaml:") || strings.HasPrefix(errStr, themeName+".yml:") || strings.HasPrefix(errStr, themeName+":") {
				return fmt.Errorf("theme '%s' exists but failed to load: %v\n\nFix the errors in your theme file and try again", themeName, err)
			}
		}
		return fmt.Errorf("unknown theme: %s\n\nRun 'claudio config theme list' to see available themes.\nCustom themes should be placed in: %s", themeName, styles.ThemesDir())
	}

	fmt.Printf("Theme: %s\n", themeName)
	fmt.Println()

	if styles.IsBuiltinTheme(themeName) {
		fmt.Println("Type: Built-in")
	} else {
		fmt.Println("Type: Custom")
		if theme := styles.GetCustomTheme(styles.ThemeName(themeName)); theme != nil {
			if theme.Author != "" {
				fmt.Printf("Author: %s\n", theme.Author)
			}
			if theme.Description != "" {
				fmt.Printf("Description: %s\n", theme.Description)
			}
		}
	}

	// Show the color palette
	palette := styles.GetPalette(styles.ThemeName(themeName))
	fmt.Println()
	fmt.Println("Base Colors:")
	fmt.Printf("  Primary:   %s\n", palette.Primary)
	fmt.Printf("  Secondary: %s\n", palette.Secondary)
	fmt.Printf("  Warning:   %s\n", palette.Warning)
	fmt.Printf("  Error:     %s\n", palette.Error)
	fmt.Printf("  Muted:     %s\n", palette.Muted)
	fmt.Printf("  Surface:   %s\n", palette.Surface)
	fmt.Printf("  Text:      %s\n", palette.Text)
	fmt.Printf("  Border:    %s\n", palette.Border)

	return nil
}

func runThemePath(cmd *cobra.Command, args []string) error {
	themesDir := styles.ThemesDir()
	fmt.Println(themesDir)

	// Check if directory exists
	if _, err := os.Stat(themesDir); os.IsNotExist(err) {
		fmt.Println()
		fmt.Println("Note: This directory does not exist yet.")
		fmt.Println("It will be created when you add your first custom theme.")
	}

	return nil
}

func runThemeCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Validate the name
	if name == "" {
		return fmt.Errorf("theme name cannot be empty")
	}

	// Check for invalid characters
	if strings.ContainsAny(name, "/\\:*?\"<>|") {
		return fmt.Errorf("theme name contains invalid characters")
	}

	// Check if it would override a built-in theme
	if styles.IsBuiltinTheme(name) {
		return fmt.Errorf("cannot create custom theme with built-in name '%s'", name)
	}

	// Check if theme already exists
	themesDir := styles.ThemesDir()
	themePath := filepath.Join(themesDir, name+".yaml")
	if _, err := os.Stat(themePath); err == nil {
		return fmt.Errorf("theme '%s' already exists at %s", name, themePath)
	}

	// Create the theme file with default palette
	theme := &styles.ThemeFile{
		Name:        capitalizeFirst(name),
		Author:      "",
		Description: "A custom Claudio theme",
		Version:     "1",
		Colors: styles.ThemeColors{
			Primary:   "#A78BFA",
			Secondary: "#10B981",
			Warning:   "#F59E0B",
			Error:     "#F87171",
			Muted:     "#9CA3AF",
			Surface:   "#1F2937",
			Text:      "#F9FAFB",
			Border:    "#6B7280",
		},
	}

	if err := styles.SaveTheme(name, theme); err != nil {
		return fmt.Errorf("creating theme: %w", err)
	}

	fmt.Printf("Created new theme: %s\n", themePath)
	fmt.Println()
	fmt.Println("Edit this file to customize your theme colors.")
	fmt.Println("The theme will be available after restarting Claudio.")
	fmt.Println()
	fmt.Printf("To use your new theme, run:\n")
	fmt.Printf("  claudio config set tui.theme %s\n", name)

	return nil
}

// capitalizeFirst capitalizes the first character of a string.
// This is a simple replacement for strings.Title which is deprecated.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
