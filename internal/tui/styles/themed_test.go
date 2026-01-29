package styles

import (
	"testing"
)

func TestNewThemedStyles(t *testing.T) {
	p := DefaultPalette()
	s := NewThemedStyles(p)

	if s == nil {
		t.Fatal("NewThemedStyles() returned nil")
	}

	// Verify colors are copied correctly
	if s.PrimaryColor != p.Primary {
		t.Errorf("PrimaryColor = %q, want %q", s.PrimaryColor, p.Primary)
	}
	if s.SecondaryColor != p.Secondary {
		t.Errorf("SecondaryColor = %q, want %q", s.SecondaryColor, p.Secondary)
	}
}

func TestThemedStyles_StatusColor(t *testing.T) {
	s := NewThemedStyles(DefaultPalette())

	tests := []struct {
		status   string
		expected string
	}{
		{"working", "#10B981"},
		{"pending", "#9CA3AF"},
		{"waiting_input", "#F59E0B"},
		{"paused", "#60A5FA"},
		{"completed", "#A78BFA"},
		{"error", "#F87171"},
		{"creating_pr", "#F472B6"},
		{"stuck", "#FB923C"},
		{"timeout", "#F87171"},
		{"interrupted", "#FBBF24"},
		{"unknown", "#9CA3AF"}, // Falls back to muted
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := s.StatusColor(tt.status)
			if string(got) != tt.expected {
				t.Errorf("StatusColor(%q) = %q, want %q", tt.status, got, tt.expected)
			}
		})
	}
}

func TestThemedStyles_SessionTypeColor(t *testing.T) {
	s := NewThemedStyles(DefaultPalette())

	tests := []struct {
		sessionType string
		expected    string
	}{
		{"plan", "#A78BFA"},       // Purple
		{"plan_multi", "#A78BFA"}, // Purple
		{"ultraplan", "#FBBF24"},  // Yellow
		{"tripleshot", "#60A5FA"}, // Blue
		{"unknown", "#9CA3AF"},    // Falls back to muted
	}

	for _, tt := range tests {
		t.Run(tt.sessionType, func(t *testing.T) {
			got := s.SessionTypeColor(tt.sessionType)
			if string(got) != tt.expected {
				t.Errorf("SessionTypeColor(%q) = %q, want %q", tt.sessionType, got, tt.expected)
			}
		})
	}
}

func TestThemedStyles_StylesCanRender(t *testing.T) {
	s := NewThemedStyles(DefaultPalette())

	// Test that all styles can render without panicking
	styles := map[string]func() string{
		"Primary":                   func() string { return s.Primary.Render("test") },
		"Secondary":                 func() string { return s.Secondary.Render("test") },
		"Warning":                   func() string { return s.Warning.Render("test") },
		"Error":                     func() string { return s.Error.Render("test") },
		"Muted":                     func() string { return s.Muted.Render("test") },
		"Surface":                   func() string { return s.Surface.Render("test") },
		"Text":                      func() string { return s.Text.Render("test") },
		"Title":                     func() string { return s.Title.Render("test") },
		"Header":                    func() string { return s.Header.Render("test") },
		"HelpBar":                   func() string { return s.HelpBar.Render("test") },
		"HelpKey":                   func() string { return s.HelpKey.Render("test") },
		"Sidebar":                   func() string { return s.Sidebar.Render("test") },
		"SidebarItem":               func() string { return s.SidebarItem.Render("test") },
		"SidebarItemActive":         func() string { return s.SidebarItemActive.Render("test") },
		"DiffAdd":                   func() string { return s.DiffAdd.Render("test") },
		"DiffRemove":                func() string { return s.DiffRemove.Render("test") },
		"DiffHeader":                func() string { return s.DiffHeader.Render("test") },
		"DiffHunk":                  func() string { return s.DiffHunk.Render("test") },
		"DiffContext":               func() string { return s.DiffContext.Render("test") },
		"SearchBar":                 func() string { return s.SearchBar.Render("test") },
		"SearchMatch":               func() string { return s.SearchMatch.Render("test") },
		"DropdownItem":              func() string { return s.DropdownItem.Render("test") },
		"DropdownItemSelected":      func() string { return s.DropdownItemSelected.Render("test") },
		"ModeBadgeNormal":           func() string { return s.ModeBadgeNormal.Render("test") },
		"ModeBadgeInput":            func() string { return s.ModeBadgeInput.Render("test") },
		"TerminalPaneBorder":        func() string { return s.TerminalPaneBorder.Render("test") },
		"TerminalPaneBorderFocused": func() string { return s.TerminalPaneBorderFocused.Render("test") },
	}

	for name, renderFunc := range styles {
		t.Run(name, func(t *testing.T) {
			// Just verify it doesn't panic
			_ = renderFunc()
		})
	}
}

func TestSetActiveTheme(t *testing.T) {
	// Store original theme to restore after test
	originalPrimary := PrimaryColor

	tests := []struct {
		theme         ThemeName
		wantPrimary   string
		wantSecondary string
	}{
		{ThemeDefault, "#A78BFA", "#10B981"},
		{ThemeMonokai, "#F92672", "#A6E22E"},
		{ThemeDracula, "#BD93F9", "#50FA7B"},
		{ThemeNord, "#88C0D0", "#A3BE8C"},
	}

	for _, tt := range tests {
		t.Run(string(tt.theme), func(t *testing.T) {
			SetActiveTheme(tt.theme)

			// Check that global colors were updated
			if string(PrimaryColor) != tt.wantPrimary {
				t.Errorf("PrimaryColor = %q, want %q", PrimaryColor, tt.wantPrimary)
			}
			if string(SecondaryColor) != tt.wantSecondary {
				t.Errorf("SecondaryColor = %q, want %q", SecondaryColor, tt.wantSecondary)
			}

			// Check that GetActiveTheme returns correct theme
			active := GetActiveTheme()
			if active == nil {
				t.Fatal("GetActiveTheme() returned nil")
			}
			if string(active.PrimaryColor) != tt.wantPrimary {
				t.Errorf("GetActiveTheme().PrimaryColor = %q, want %q", active.PrimaryColor, tt.wantPrimary)
			}
		})
	}

	// Restore default theme
	SetActiveTheme(ThemeDefault)
	if PrimaryColor != originalPrimary {
		// If original was also default, this is fine
		if string(originalPrimary) != "#A78BFA" {
			t.Logf("Note: Original PrimaryColor was %q", originalPrimary)
		}
	}
}

func TestGetActiveTheme(t *testing.T) {
	active := GetActiveTheme()
	if active == nil {
		t.Fatal("GetActiveTheme() returned nil")
	}

	// Active theme should have colors set
	if active.PrimaryColor == "" {
		t.Error("Primary color should be set")
	}
	if active.SecondaryColor == "" {
		t.Error("Secondary color should be set")
	}

	// Verify styles can render (they're valid)
	_ = active.Primary.Render("test")
	_ = active.Secondary.Render("test")
}

func TestThemedStylesForAllPalettes(t *testing.T) {
	themes := []ThemeName{ThemeDefault, ThemeMonokai, ThemeDracula, ThemeNord}

	for _, theme := range themes {
		t.Run(string(theme), func(t *testing.T) {
			p := GetPalette(theme)
			s := NewThemedStyles(p)

			// Verify all essential colors are set
			if s.PrimaryColor == "" {
				t.Error("Primary color not set")
			}
			if s.SecondaryColor == "" {
				t.Error("Secondary color not set")
			}

			// Verify styles can render (they're valid)
			_ = s.Primary.Render("test")
			_ = s.Secondary.Render("test")
			_ = s.Header.Render("test")
			_ = s.DiffAdd.Render("test")
			_ = s.DiffRemove.Render("test")

			// Verify the themed styles use the palette colors
			if s.PrimaryColor != p.Primary {
				t.Errorf("PrimaryColor mismatch: got %q, want %q", s.PrimaryColor, p.Primary)
			}
			if s.SecondaryColor != p.Secondary {
				t.Errorf("SecondaryColor mismatch: got %q, want %q", s.SecondaryColor, p.Secondary)
			}
		})
	}
}

func TestSyncGlobalStyles(t *testing.T) {
	// Set to monokai and verify globals change
	SetActiveTheme(ThemeMonokai)

	if string(PrimaryColor) != "#F92672" {
		t.Errorf("After SetActiveTheme(Monokai), PrimaryColor = %q, want #F92672", PrimaryColor)
	}

	// Verify styles also updated
	rendered := Primary.Render("x")
	if rendered == "" {
		t.Error("Primary style should render content")
	}

	// Reset to default
	SetActiveTheme(ThemeDefault)
	if string(PrimaryColor) != "#A78BFA" {
		t.Errorf("After SetActiveTheme(Default), PrimaryColor = %q, want #A78BFA", PrimaryColor)
	}
}
