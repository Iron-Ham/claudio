# Keyboard Shortcuts

Quick reference for TUI keyboard shortcuts.

## Instance Selection

| Key | Action |
|-----|--------|
| `Tab` | Next instance |
| `Shift+Tab` | Previous instance |
| `l` / `→` | Next instance |
| `h` / `←` | Previous instance |

## Instance Control

| Key | Action |
|-----|--------|
| `a` | Add new instance |
| `s` | Start selected instance |
| `p` | Pause/resume instance |
| `x` | Stop instance (with PR workflow) |
| `Enter` | Focus instance for input |
| `Esc` | Exit input mode / close dialogs |

## Output Navigation

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll down |
| `k` / `↑` | Scroll up |
| `Ctrl+d` | Page down |
| `Ctrl+u` | Page up |
| `g` | Jump to top |
| `G` | Jump to bottom (latest) |

## Group Commands (g prefix)

These shortcuts use a vim-style `g` prefix. Press `g` first, then the action key.

| Key | Action |
|-----|--------|
| `gc` | Collapse/expand current group |
| `gC` | Collapse/expand all groups |
| `gn` | Jump to next group |
| `gp` | Jump to previous group |
| `gs` | Skip current group (mark pending as skipped) |
| `gr` | Retry failed tasks in current group |
| `gf` | Force-start next group (ignore dependencies) |

> **Note:** Group commands require `experimental.grouped_instance_view: true` in your config.

## Views

| Key | Action |
|-----|--------|
| `d` | Toggle diff preview |
| `c` | Toggle conflict view |
| `?` | Toggle help overlay |

## Search

| Key | Action |
|-----|--------|
| `/` | Open search |
| `n` | Next match |
| `N` | Previous match |
| `Esc` | Clear search |

## Input Mode

| Key | Action |
|-----|--------|
| `←` / `→` | Move cursor |
| `Ctrl+a` | Beginning of line |
| `Ctrl+e` | End of line |
| `Ctrl+k` | Delete to end |
| `Ctrl+u` | Delete to beginning |
| `Backspace` | Delete character |
| `Enter` | Send input |
| `Esc` | Cancel input |

## Global

| Key | Action |
|-----|--------|
| `q` | Quit Claudio |
| `?` | Toggle help |
| `:` | Enter command mode |
| `Esc` | Exit command mode / close dialogs |

## Command Mode

Press `:` to enter command mode, then type a command:

| Command | Action |
|---------|--------|
| `:plan "..."` | Inline plan generation |
| `:ultraplan "..."` | Inline UltraPlan workflow |
| `:multiplan "..."` | Multi-pass planning |
| `:tripleshot "..."` | TripleShot execution |
| `:group create` | Create new group |
| `:group add` | Add instance to group |
| `:group show` | Toggle grouped view |
| `:q!` | Force quit with cleanup |

## Status Indicators

| Symbol | State |
|--------|-------|
| `▶` | Working |
| `⏸` | Paused |
| `⏳` | Waiting for input |
| `✓` | Completed |
| `■` | Stopped |
| `○` | Pending |
