# TUI Navigation

Claudio's Terminal User Interface (TUI) provides a real-time dashboard for managing your parallel Claude instances.

## Layout Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Claudio - my-feature                                           [?] Help │
├─────────────────────┬───────────────────────────────────────────────────┤
│                     │                                                   │
│  Instance Sidebar   │              Output Panel                         │
│                     │                                                   │
│  ▶ 1. auth-api      │  Claude is working on implementing               │
│    [working]        │  the authentication endpoint...                   │
│    $0.12 | 5.2k     │                                                   │
│                     │  I'll start by creating the user model...         │
│    2. tests         │                                                   │
│    [waiting_input]  │  > Created src/models/user.ts                     │
│    $0.08 | 3.1k     │  > Created src/routes/auth.ts                     │
│                     │                                                   │
│    3. docs          │                                                   │
│    [completed]      │                                                   │
│    $0.05 | 2.0k     │                                                   │
│                     │                                                   │
├─────────────────────┴───────────────────────────────────────────────────┤
│ [a]dd [s]tart [p]ause [x]stop [d]iff [c]onflicts [/]search [?]help [q]uit│
└─────────────────────────────────────────────────────────────────────────┘
```

## Instance Selection

| Key | Action |
|-----|--------|
| `Tab` | Next instance |
| `Shift+Tab` | Previous instance |
| `l` or `→` | Next instance |
| `h` or `←` | Previous instance |
| `j` or `↓` | Scroll sidebar down (when many instances) |
| `k` or `↑` | Scroll sidebar up |

## Instance Control

| Key | Action |
|-----|--------|
| `a` | Add new instance |
| `s` | Start selected instance |
| `p` | Pause/resume instance |
| `x` | Stop instance (prompts for PR workflow) |
| `Enter` | Focus instance for input |
| `Esc` | Exit input mode |

## Output Navigation

| Key | Action |
|-----|--------|
| `j` or `↓` | Scroll down |
| `k` or `↑` | Scroll up |
| `Ctrl+d` | Page down |
| `Ctrl+u` | Page up |
| `g` | Jump to top |
| `G` | Jump to bottom (latest output) |

## Views and Panels

| Key | Action |
|-----|--------|
| `d` | Toggle diff preview panel |
| `c` | Toggle conflict view |
| `?` | Toggle help overlay |

### Diff Panel

Press `d` to see a live diff of changes made by the selected instance:

```
┌─ Diff: auth-api ─────────────────────────────────────────────────┐
│ src/routes/auth.ts                                                │
│ + import { Router } from 'express';                               │
│ + import { validateCredentials } from '../utils/auth';            │
│ +                                                                 │
│ + const router = Router();                                        │
│ +                                                                 │
│ + router.post('/login', async (req, res) => {                     │
│ +   const { email, password } = req.body;                         │
└───────────────────────────────────────────────────────────────────┘
```

### Conflict View

Press `c` to see files modified by multiple instances:

```
┌─ File Conflicts ─────────────────────────────────────────────────┐
│                                                                   │
│ src/config.ts                                                     │
│   Modified by: auth-api (abc123), tests (def456)                  │
│                                                                   │
│ No conflicts detected in other files.                             │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

## Search and Filter

### Basic Search

Press `/` to open the search prompt:

```
Search: error
```

Matching lines are highlighted in the output.

### Navigation

| Key | Action |
|-----|--------|
| `n` | Next match |
| `N` | Previous match |
| `Esc` | Clear search |

### Regex Patterns

Search supports regular expressions:

```
/error.*timeout          # Errors mentioning timeout
/\[ERROR\]               # Literal [ERROR] tags
/func\s+\w+              # Function definitions
```

## Input Mode

When Claude needs input, press `Enter` to focus:

1. Type your response
2. Press `Enter` to send
3. Press `Esc` to cancel

The input field supports editing:

| Key | Action |
|-----|--------|
| `←` / `→` | Move cursor |
| `Ctrl+a` | Beginning of line |
| `Ctrl+e` | End of line |
| `Ctrl+k` | Delete to end of line |
| `Ctrl+u` | Delete to beginning |
| `Backspace` | Delete character |

## Command Mode

Press `:` to enter command mode for advanced operations. Type a command and press `Enter` to execute.

### Available Commands

| Command | Description |
|---------|-------------|
| `:plan "objective"` | Start inline plan generation |
| `:ultraplan "objective"` | Start inline UltraPlan workflow |
| `:multiplan "objective"` | Multi-pass planning (requires `experimental.inline_plan`) |
| `:tripleshot "task"` | Start TripleShot execution (requires `experimental.triple_shot`) |
| `:group create [name]` | Create a new instance group |
| `:group add <instance> <group>` | Add instance to a group |
| `:group remove <instance>` | Remove instance from its group |
| `:group move <instance> <group>` | Move instance to a different group |
| `:group order <g1,g2,g3>` | Reorder group execution sequence |
| `:group delete [name]` | Delete an empty group |
| `:group show` | Toggle grouped view on/off |
| `:q!` or `:quit!` | Force quit with cleanup |

### Example Workflow

```
:plan "Implement user authentication with OAuth2"
```

This starts the inline planning workflow:
1. Claude analyzes the objective
2. A structured plan is generated
3. You review and edit tasks in the plan editor
4. Confirm to spawn instances organized by groups

### Enabling Experimental Commands

Some commands require experimental flags in your config:

```yaml
# ~/.config/claudio/config.yaml
experimental:
  inline_plan: true       # Enables :multiplan
  inline_ultraplan: true  # Enables :ultraplan
  triple_shot: true       # Enables :tripleshot
  grouped_instance_view: true  # Enables :group commands
```

---

## Adding Instances

Press `a` to add a new instance:

```
┌─ Add Instance ───────────────────────────────────────────────────┐
│                                                                   │
│ Task: Implement password reset functionality                      │
│                                                                   │
│ Press Enter to create, Esc to cancel                              │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

Tips for task descriptions:
- Be specific about what you want
- Mention relevant files or areas
- Include acceptance criteria if helpful

## Status Indicators

### Instance States

| Indicator | State | Meaning |
|-----------|-------|---------|
| `▶` | working | Actively processing |
| `⏸` | paused | Manually paused |
| `⏳` | waiting_input | Needs user input |
| `✓` | completed | Task finished |
| `■` | stopped | Manually stopped |
| `○` | pending | Not yet started |

### Colors

The sidebar uses colors to indicate state:
- **Green**: Active/working
- **Yellow**: Waiting for input
- **Blue**: Completed
- **Gray**: Paused/stopped
- **Red**: Error state

### Metrics Display

When enabled, the sidebar shows:
```
1. auth-api
   [working]
   $0.12 | 5.2k tokens
```

- `$0.12` - Estimated cost
- `5.2k` - Token count (input + output)

## Quitting

Press `q` to quit. You'll see options:

```
┌─ Quit Claudio ───────────────────────────────────────────────────┐
│                                                                   │
│ You have 2 running instances. What would you like to do?          │
│                                                                   │
│ [1] Stop all instances and quit                                   │
│ [2] Keep instances running in background                          │
│ [3] Cancel                                                        │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

If you keep instances running, recover later with:
```bash
claudio sessions recover
```

## Keyboard Reference

### Global

| Key | Action |
|-----|--------|
| `q` | Quit |
| `?` | Toggle help |
| `Esc` | Close dialogs/clear search |

### Navigation

| Key | Action |
|-----|--------|
| `Tab`/`Shift+Tab` | Next/previous instance |
| `h`/`l` | Previous/next instance |
| `j`/`k` | Scroll output |
| `g`/`G` | Top/bottom of output |
| `Ctrl+d`/`Ctrl+u` | Page down/up |

### Actions

| Key | Action |
|-----|--------|
| `a` | Add instance |
| `s` | Start instance |
| `p` | Pause/resume |
| `x` | Stop instance |
| `Enter` | Input mode |

### Views

| Key | Action |
|-----|--------|
| `d` | Diff panel |
| `c` | Conflict view |
| `/` | Search |
| `n`/`N` | Next/prev match |
