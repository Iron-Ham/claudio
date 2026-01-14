package tui

import "testing"

func TestCalculateContentDimensions(t *testing.T) {
	tests := []struct {
		name       string
		termWidth  int
		termHeight int
		wantWidth  int
		wantHeight int
	}{
		{
			name:       "standard terminal uses full sidebar width",
			termWidth:  120,
			termHeight: 40,
			wantWidth:  120 - SidebarWidth - ContentWidthOffset, // 120 - 30 - 7 = 83
			wantHeight: 40 - ContentHeightOffset,                // 40 - 12 = 28
		},
		{
			name:       "narrow terminal uses minimum sidebar width",
			termWidth:  79,
			termHeight: 30,
			wantWidth:  79 - SidebarMinWidth - ContentWidthOffset, // 79 - 20 - 7 = 52
			wantHeight: 30 - ContentHeightOffset,                  // 30 - 12 = 18
		},
		{
			name:       "exactly 80 width uses full sidebar",
			termWidth:  80,
			termHeight: 24,
			wantWidth:  80 - SidebarWidth - ContentWidthOffset, // 80 - 30 - 7 = 43
			wantHeight: 24 - ContentHeightOffset,               // 24 - 12 = 12
		},
		{
			name:       "very narrow terminal",
			termWidth:  60,
			termHeight: 20,
			wantWidth:  60 - SidebarMinWidth - ContentWidthOffset, // 60 - 20 - 7 = 33
			wantHeight: 20 - ContentHeightOffset,                  // 20 - 12 = 8
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWidth, gotHeight := CalculateContentDimensions(tt.termWidth, tt.termHeight)
			if gotWidth != tt.wantWidth {
				t.Errorf("CalculateContentDimensions() width = %v, want %v", gotWidth, tt.wantWidth)
			}
			if gotHeight != tt.wantHeight {
				t.Errorf("CalculateContentDimensions() height = %v, want %v", gotHeight, tt.wantHeight)
			}
		})
	}
}

func TestCalculateEffectiveSidebarWidth(t *testing.T) {
	tests := []struct {
		name      string
		termWidth int
		want      int
	}{
		{
			name:      "standard terminal returns full width",
			termWidth: 120,
			want:      SidebarWidth,
		},
		{
			name:      "narrow terminal returns minimum width",
			termWidth: 79,
			want:      SidebarMinWidth,
		},
		{
			name:      "exactly 80 returns full width",
			termWidth: 80,
			want:      SidebarWidth,
		},
		{
			name:      "very narrow terminal",
			termWidth: 40,
			want:      SidebarMinWidth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateEffectiveSidebarWidth(tt.termWidth)
			if got != tt.want {
				t.Errorf("CalculateEffectiveSidebarWidth(%d) = %v, want %v", tt.termWidth, got, tt.want)
			}
		})
	}
}

func TestLayoutConstants(t *testing.T) {
	// Verify constants have expected values to catch accidental changes
	if SidebarWidth != 30 {
		t.Errorf("SidebarWidth = %d, want 30", SidebarWidth)
	}
	if SidebarMinWidth != 20 {
		t.Errorf("SidebarMinWidth = %d, want 20", SidebarMinWidth)
	}
	if ContentWidthOffset != 7 {
		t.Errorf("ContentWidthOffset = %d, want 7", ContentWidthOffset)
	}
	if ContentHeightOffset != 12 {
		t.Errorf("ContentHeightOffset = %d, want 12", ContentHeightOffset)
	}
}
