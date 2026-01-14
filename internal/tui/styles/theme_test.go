package styles

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewTheme(t *testing.T) {
	theme := NewTheme()
	if theme == nil {
		t.Error("NewTheme() returned nil")
	}
}

func TestTheme_ImplementsInterface(t *testing.T) {
	// This test verifies that Theme implements the panel.Theme interface
	// by checking all required methods return valid lipgloss.Style values
	theme := NewTheme()

	methods := []struct {
		name   string
		method func() lipgloss.Style
	}{
		{"Primary", theme.Primary},
		{"Secondary", theme.Secondary},
		{"Muted", theme.Muted},
		{"Error", theme.Error},
		{"Warning", theme.Warning},
		{"Surface", theme.Surface},
		{"Border", theme.Border},
		{"DiffAdd", theme.DiffAdd},
		{"DiffRemove", theme.DiffRemove},
		{"DiffHeader", theme.DiffHeader},
		{"DiffHunk", theme.DiffHunk},
		{"DiffContext", theme.DiffContext},
	}

	for _, m := range methods {
		t.Run(m.name, func(t *testing.T) {
			style := m.method()
			// Verify style can be rendered (validates it's a valid Style)
			_ = style.Render("test")
		})
	}
}

func TestTheme_StyleValues(t *testing.T) {
	theme := NewTheme()

	// Test that certain theme methods return expected styles
	t.Run("Primary returns Primary style", func(t *testing.T) {
		got := theme.Primary().Render("test")
		want := Primary.Render("test")
		if got != want {
			t.Errorf("Primary() rendered %q, want %q", got, want)
		}
	})

	t.Run("Secondary returns HelpKey style", func(t *testing.T) {
		got := theme.Secondary().Render("test")
		want := HelpKey.Render("test")
		if got != want {
			t.Errorf("Secondary() rendered %q, want %q", got, want)
		}
	})

	t.Run("Muted returns Muted style", func(t *testing.T) {
		got := theme.Muted().Render("test")
		want := Muted.Render("test")
		if got != want {
			t.Errorf("Muted() rendered %q, want %q", got, want)
		}
	})

	t.Run("Error returns Error style", func(t *testing.T) {
		got := theme.Error().Render("test")
		want := Error.Render("test")
		if got != want {
			t.Errorf("Error() rendered %q, want %q", got, want)
		}
	})

	t.Run("Warning returns Warning style", func(t *testing.T) {
		got := theme.Warning().Render("test")
		want := Warning.Render("test")
		if got != want {
			t.Errorf("Warning() rendered %q, want %q", got, want)
		}
	})

	t.Run("Surface returns Surface style", func(t *testing.T) {
		got := theme.Surface().Render("test")
		want := Surface.Render("test")
		if got != want {
			t.Errorf("Surface() rendered %q, want %q", got, want)
		}
	})

	t.Run("DiffHeader returns Primary style", func(t *testing.T) {
		got := theme.DiffHeader().Render("test")
		want := Primary.Render("test")
		if got != want {
			t.Errorf("DiffHeader() rendered %q, want %q", got, want)
		}
	})

	t.Run("DiffContext returns Muted style", func(t *testing.T) {
		got := theme.DiffContext().Render("test")
		want := Muted.Render("test")
		if got != want {
			t.Errorf("DiffContext() rendered %q, want %q", got, want)
		}
	})
}

func TestTheme_DiffColors(t *testing.T) {
	theme := NewTheme()

	t.Run("Border uses BorderColor", func(t *testing.T) {
		expected := lipgloss.NewStyle().Foreground(BorderColor).Render("test")
		got := theme.Border().Render("test")
		if got != expected {
			t.Errorf("Border() rendered %q, want %q", got, expected)
		}
	})

	t.Run("DiffAdd uses GreenColor", func(t *testing.T) {
		expected := lipgloss.NewStyle().Foreground(GreenColor).Render("test")
		got := theme.DiffAdd().Render("test")
		if got != expected {
			t.Errorf("DiffAdd() rendered %q, want %q", got, expected)
		}
	})

	t.Run("DiffRemove uses RedColor", func(t *testing.T) {
		expected := lipgloss.NewStyle().Foreground(RedColor).Render("test")
		got := theme.DiffRemove().Render("test")
		if got != expected {
			t.Errorf("DiffRemove() rendered %q, want %q", got, expected)
		}
	})

	t.Run("DiffHunk uses BlueColor", func(t *testing.T) {
		expected := lipgloss.NewStyle().Foreground(BlueColor).Render("test")
		got := theme.DiffHunk().Render("test")
		if got != expected {
			t.Errorf("DiffHunk() rendered %q, want %q", got, expected)
		}
	})
}
