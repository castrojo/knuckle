# Subiquity TUI Design Analysis — Patterns for knuckle

> Sourced from: `canonical/subiquity` — `subiquity/ui/views/`, `subiquitycore/ui/`
> Analyzed: `form.py`, `utils.py`, `anchors.py`, `frame.py`, `palette.py`, `spinner.py`,
> `stretchy.py`, `buttons.py`, `confirmation.py`, `identity.py`, `installprogress.py`,
> `snaplist.py`, `filesystem/guided.py`, `views/network.py`, `views/error.py`, `DESIGN.md`
> Date: 2026-05-19

---

## Executive Summary

Subiquity is a production-grade TUI installer that runs in 80×24 terminals on bare metal
and serial lines. It has been battle-tested across millions of Ubuntu Server installs. Its
design philosophy maps almost perfectly onto what knuckle needs. The core ideas:

1. **No sidebar, no breadcrumb** — header-only step labelling with title changes
2. **`excerpt → scrollable content → button pile`** is the canonical screen layout
3. **Two-column label/input table** with one help-text row per field
4. **Validation is immediate, inline, and blocks the Done button**
5. **Destructive confirmation is a modal overlay** (Stretchy), never inline
6. **Progress = spinner-per-event in a scrollable LineBox**, not a global progress bar
7. **Error text is always `danger` color inline** — no toast, no banner elsewhere
8. **8 colors only** (linux tty constraint), palette drives all visual distinction

---

## 1. Step Flow and Navigation

### What subiquity does

**No sidebar.** No breadcrumb. No step counter. Zero decorative navigation chrome.

Navigation is purely sequential:
- **Forward**: "Done" button at bottom of each screen
- **Back**: "Back" button at bottom, labeled with `cancel_label = "Back"`
- The screen title is the only navigation context provided

The `SubiquityCoreUI` frame (`subiquitycore/ui/frame.py`) has:
- A **header** (3 lines tall: fringe + title + fringe) — always visible, always reflects current step
- A **body** that swaps entirely when moving between screens

```python
# frame.py — the whole frame
def set_body(self, widget):
    self.set_header(_(widget.title))   # title from the view's .title attr
    self._assign_contents(1, widget)   # replaces body wholesale
```

The header (`anchors.py`) renders as three rows:
```
▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄  ← LOWER HALF BLOCK characters, orange bg
  Profile configuration  ← white text on ubuntu orange (#E95420)
▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀  ← UPPER HALF BLOCK characters, orange bg
```

The title is left-positioned within a 79%-width center region, with a help button
(`right_icon`) docked to the right end of that same region.

### Knuckle implications

| Subiquity pattern | Knuckle implementation |
|---|---|
| Title-only navigation | `lipgloss` styled header bar, title changes per step |
| No sidebar/breadcrumb | Correct — don't add one. Wastes vertical space on 80×24 |
| Back = button at bottom | `huh.Form` cancel_label="Back"; or explicit Back button in raw BubbleTea |
| Forward = "Done" button | `huh.Form` confirms; or explicit Done button |
| Header height = 3 lines | Allocate exactly 3 rows: fringe + title + fringe; leave rest for content |
| Back is always safe | Never require re-validation to go back. Back should always work |

**Design constraint from DESIGN.md:** "Subiquity is functional in an 80×24 terminal."
This means the form content area is realistically ~18 rows (24 - 3 header - 1 blank - 2 button area).
Every step must fit (or scroll gracefully) in that space.

---

## 2. Form Layout

### What subiquity does

Every form screen follows the `screen()` helper pattern from `subiquitycore/ui/utils.py`:

```
[ 1-line blank ]
excerpt (plain text describing the form's purpose)
[ 1-line blank ]
╔═══════════════════════════════════════════════════════════════╗
║  [scrollable ListBox with form rows]                         ║
╚═══════════════════════════════════════════════════════════════╝
[ 1-line blank ]
           [  Done  ]   [  Cancel  ]
[ 1-line blank ]
```

Total width: `center_79` = 79% of terminal width, minimum 76 columns.
Narrow variant (`narrow_rows=True`): `center_63` = 63% of terminal width.

**The two-column field table** (from `form.py`):

```
                Your name:  ┌──────────────────────────────┐
                            └──────────────────────────────┘
                            (help text in gray — "info_minor")

         Your server's name: ┌──────────────────────────────┐
                             └──────────────────────────────┘
                             The name it uses when it talks...

                             (blank line between fields)

           Pick a username: ┌──────────────────────────────┐
                            └──────────────────────────────┘
```

Each field is a **2-row table**:
- Row 0: `[right-aligned caption]  [input widget]` — 2 column spacing
- Row 1: `[empty]  [help text or error text]` — occupies 1 line always (even if no help)

Between fields there is always one blank `Text("")` separator.

Column layout uses `ColSpec(pack=False)` for col 1 (input widget, expands), and auto-pack for col 0 (caption).
Caption alignment: **right-aligned** (so all inputs are left-edge-aligned).

Input widget styling:
- Unfocused: `string_input` = black text on **white** background (inverted)
- Focused: `string_input focus` = white text on **gray** background

Checkbox and RadioButton fields are reversed (caption is to the **right** of the widget):
```
   [*] Set up this disk as an LVM group    ← caption_first = False
```

Buttons: min 14 chars wide, centered, stacked vertically in the center of the screen:
```
           [  Done  ]
           [  Back  ]
```

Button semantic colors on focus:
- Done: **green** background (`done_button focus` = fg on `good`)
- Cancel/Back: **gray** background (`other_button focus`)
- Danger (destructive confirm): **red** background (`danger_button focus`)

### Knuckle implications

| Subiquity pattern | Knuckle implementation |
|---|---|
| `caption: [right-aligned]` + `[input widget]` table | `huh.Form` fields with caption — huh handles this layout |
| Help text row always present | `huh` field `.Description()` — always renders, even if empty |
| Blank line between fields | huh handles this naturally |
| Validation error in help row | `huh` inline error display matches this exactly |
| Content width = 79% terminal | Set `huh.Form` with `Width(termWidth * 79 / 100)` or use lipgloss padding |
| Focused input: different bg | huh's built-in focus styling handles this |
| Done button = green on focus | Use `lipgloss` to style the confirm button with `ActiveStyles` |
| Back button = gray on focus | Use `lipgloss` secondary button style |
| Destructive button = red | Use `lipgloss` danger style for the confirm button on the final step |
| Excerpt above form | Add a `lipgloss.NewStyle().Foreground(...)` text above the `huh.Form` in the view |

**Critical:** `huh.Form` already implements most of this. The caption/input/help-text pattern
is huh's default field rendering. Knuckle should lean into huh's defaults rather than
fighting them.

---

## 3. Selection Screens

### What subiquity does

#### Disk selection (single choice, mutually exclusive)

From `filesystem/guided.py`, the disk selector uses a `Selector` widget (like `<select>`)
combined with a `ChoiceField`. Each option is a mini-table showing disk metadata.

Visual structure of a disk option:
```
  sda       disk         Samsung SSD 860    250.1 GB
              sda1  EFI System Partition    512.0 MB
              sda2  ext4, /                249.5 GB

  sdb       disk         WD Blue            1.0 TB
            (not formatted)
```

- Disk type and size right-aligned
- Partition annotations shown in gray (`info_minor`)
- Usage labels dimmed if disk has no partitions
- Currently selected disk gets highlighted (Selector handles this)
- The entire disk-choice widget is scrollable if many disks

For the guided form, radio buttons appear as:
```
  ( ) Use an entire disk        ← StarRadioButton: ( ) / (*)
      [ disk picker sub-form ]
  ( ) Custom storage layout
```

Subforms are indented under their parent radio button.

#### Multi-select (snap list analog → sysexts)

From `snaplist.py`, the multi-select list:
- Each row: `[ ] snap-name   publisher ✓   summary (clipped)    ▸`
- `[ ]` = unchecked, `[*]` = checked — custom `SnapCheckBox`
- Column layout: checkbox | publisher | summary (flex, min 40) | `▸` indicator
- SPACE toggles the checkbox
- ENTER opens a detail view for the item
- Excerpt explains controls: "Select or deselect with SPACE, press ENTER to see more details"
- Items are scrollable inside a `NoTabCyclingTableListBox`
- If the list fails to load (network error): shows "Try again" / "Continue" options

The excerpt is critical — without it the key SPACE/ENTER bindings are invisible to users.

#### Mutually exclusive options (ChoiceField / Selector)

The `Selector` widget (`selector.py`) opens a dropdown-like popup when activated.
RadioButtonField groups use standard `( )` / `(*)` rendered with `SelectableIcon`.

### Knuckle implications

| Subiquity pattern | Knuckle mapping |
|---|---|
| Disk table: name \| type \| size (right-aligned) | `bubbles/table` with 3-4 cols, size right-aligned |
| Disk sub-details shown indented | Show partitions/usage below disk row, dimmed |
| Currently selected row highlighted | `bubbles/table` default selected-row styling |
| Radio buttons `( )` / `(*)` for mutually exclusive | `huh.Select` (renders as navigable list) |
| Checkboxes `[ ]` / `[*]` for multi-select | `huh.MultiSelect` |
| SPACE = toggle, ENTER = detail | bubbles table custom keybindings; detail view as overlay |
| Excerpt explains keybindings | Always render a help line: "↑↓ navigate · SPACE select · ENTER confirm" |
| Loading spinner while probing | Show spinner + "Detecting disks..." while `probe()` runs |
| No disks found: explicit error | Show "No disks detected" message + Back button |

**Specific for knuckle disk picker:**
- Show: dev path (by-id), model, serial, size, transport type, removable flag
- Highlight removable disks differently (e.g., dim them or add a `(removable)` tag)
- Never show the raw `/dev/sda` path as the primary identifier — show by-id

**Specific for knuckle sysext multi-select:**
- Column layout: `[ ] sysext-name   version   description (clipped)`
- Excerpt: "SPACE to toggle · ENTER for details · done when ready"
- If catalog fetch fails: "Failed to load sysext catalog. Try again?" with retry button

---

## 4. Confirmation / Review

### What subiquity does

The final confirmation before destructive action is an **`InstallConfirmation` Stretchy
overlay** (`installprogress.py`). This is a modal dialog centered on the screen.

```
╔════════════════ Confirm destructive action ═══════════════════╗
║                                                               ║
║  Selecting Continue below will begin the installation process ║
║  and result in the loss of data on the disks selected to be  ║
║  formatted.                                                   ║
║                                                               ║
║  You will not be able to return to this or a previous screen  ║
║  once the installation has started.                           ║
║                                                               ║
║  Are you sure you want to continue?                           ║
║                                                               ║
║                    [  No       ]                              ║
║                    [  Continue ]                              ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
```

Key design decisions:
1. **LineBox border** with title — the dialog is visually separate from everything behind it
2. **Safe choice first**: "No" (`cancel_btn`) appears BEFORE "Continue" (`danger_btn`)
3. **Focus lands on buttons** by default (`focus_index=2`)
4. **Explicit no-return statement**: "You will not be able to return to this or a previous screen once the installation has started."
5. **Danger button styling**: "Continue" uses `danger_btn` which renders red on focus
6. The text uses `rewrap()` to reformat the multiline string — no hard line breaks
7. The overlay is centered: 3px padding left/right, 1px top/bottom, max 80 cols wide

The `ConfirmationOverlay` (`subiquitycore/ui/confirmation.py`) is a reusable version for
general yes/no confirmations — same structure.

There is **no "review screen" before the confirmation**. The user configures everything,
presses "Done" on the final configuration screen, and the confirmation overlay appears
directly.

### Knuckle implications

| Subiquity pattern | Knuckle mapping |
|---|---|
| Separate modal overlay for confirmation | `huh.Confirm` or a raw BubbleTea overlay |
| LineBox border + title "Confirm destructive action" | `lipgloss.NewStyle().Border(lipgloss.RoundedBorder())` |
| Safe choice first (No → Continue) | Put "Back" / "Cancel" BEFORE "Install" in the button pile |
| Danger button = red | `lipgloss` active style: `lipgloss.Color("#FF0000")` bg for Install button |
| "Cannot return" explicit warning | Include verbatim text: "This will erase all data on {disk}. You cannot undo this." |
| Focus on buttons by default | In BubbleTea model, set initial focus to button area |
| `rewrap()` for paragraph text | Use `wordwrap` or `lipgloss` text wrapping — no manual `\n` |

**What knuckle's confirmation overlay should say:**
```
╔══════════════ Confirm installation ══════════════╗
║                                                  ║
║  Installing Flatcar to /dev/disk/by-id/...       ║
║  will erase all existing data on that disk.      ║
║                                                  ║
║  This action cannot be undone.                   ║
║  You will not be able to go back once started.   ║
║                                                  ║
║              [  Cancel  ]                        ║
║              [  Install ]   ← red when focused   ║
║                                                  ║
╚══════════════════════════════════════════════════╝
```

---

## 5. Progress During Install

### What subiquity does

`ProgressView` (`installprogress.py`) is the install progress screen. It does NOT use
a horizontal progress bar. Instead it uses:

1. **A scrolling event log** — a `ListBox` inside a `MyLineBox` with a dynamic title
2. **Per-event spinners** — each active operation has an animated spinner inline
3. **A status line** — the LineBox title (`event_linebox.set_title(text)`) shows current state

The event log looks like this (while running):
```
╔════════════════ Installing system ════════════════╗
║                                                   ║
║  Running curtin install step ...  •----            ║
║  Formatting /dev/sda1            done             ║
║  Extracting rootfs ...           |•---|            ║
║  Installing bootloader ...       |--•--|           ║
║                                                   ║
╚═══════════════════════════════════════════════════╝
              [ View full log ]
```

Two spinner styles:
- `"spin"`: `-`, `\`, `|`, `/` — 100ms interval, used inline in events
- `"dots"`: `|•----|`, `|-•---|`, `|--•--|`, `|---•-|`, `|----•|` — 200ms interval

Events use the "dots" style:
```python
Columns([
    ("pack", Text(message)),
    ("pack", spinner),   # ← dots-style, removed when event finishes
], dividechars=1)
```

When an event finishes, the spinner column is removed — the line collapses to just the message.

**Button state machine:**
- RUNNING: `[ View full log ]`
- DONE: `[ View full log ] [ Reboot Now ]`
- ERROR: `[ View full log ] [ View error report ] [ Reboot Now ]`

The full log is a second `ListBox` with raw installer output. There's a toggle button
to switch between the event view and the full log view — the screen replaces its own `_w`.

**Auto-scroll behavior:** New lines are always appended. If the user was focused on the
last item, focus tracks to the new last item. If they've scrolled up, they stay there.

The `urwid.ProgressBar` widget is only used in the error report upload indicator, not
in the main progress view.

### Knuckle implications

| Subiquity pattern | Knuckle mapping |
|---|---|
| No horizontal progress bar | Use event log + spinner per operation |
| Scrolling ListBox in a LineBox | `lipgloss.NewStyle().Border(...)` box + `viewport` component |
| Per-event inline spinner | `bubbles/spinner` per active operation in the list |
| Spinner stops when op completes | Remove spinner from completed lines |
| Dynamic title = current status | Update header/box title on each state transition |
| "View full log" button | Toggle between structured view and raw log output |
| Auto-scroll to bottom | Use `viewport.GotoBottom()` if user is at bottom |
| State machine for button set | Switch buttons based on install state: running / done / error |

**Install state titles for knuckle:**
- `"Writing Flatcar to disk..."` — during dd/flatcar-install
- `"Configuring system..."` — during post-install config
- `"Installation complete!"` — on success
- `"Installation failed"` — on error

**Key DESIGN.md rule for async operations:**
> "If something takes more than about 0.1s, it is done in the background, possibly with some
> kind of indication in the UI and the ability to cancel if appropriate. If indication is shown,
> it is shown for at least 1s to avoid flickering the UI."

This means: if an operation is fast (< 0.1s), show nothing. If slow, show spinner for
minimum 1s even if it finishes sooner (avoids flicker).

The `wait_with_text_dialog` helper implements this: shows a loading dialog only after 0.1s,
shown for at least 1s. Knuckle should implement the same pattern in `internal/runner`.

---

## 6. Error Handling

### What subiquity does

#### Field validation errors (form.py)

Errors are shown **in the help row below the field** — the same row used for help text.
Help text is replaced by error text.

```
           Pick a username:  ┌─────────────────────────┐
                             └─────────────────────────┘
                             ↑ error_text in red (info_error color)
```

Mechanism:
- `BoundFormField.show_extra(("info_error", message))` sets the under_text widget
- `validate()` is called: (a) on focus loss, (b) on character change if already in error
- Done button is **disabled** (not just grayed) while any field `in_error == True`
- Character-level validation (invalid chars): rejected immediately with inline message,
  the character never enters the field
- The error message is the return value from `validate_<fieldname>()` on the form class

**Specific validators in identity.py:**
```python
def validate_hostname(self):
    if len(self.hostname.value) < 1:
        return _("Server name must not be empty")
    if len(self.hostname.value) > HOSTNAME_MAXLEN:
        return _("Server name too long, must be less than {limit}").format(...)
    if not re.match(HOSTNAME_REGEX, self.hostname.value):
        return _("Hostname must match ...")
```

#### Network errors (network.py)

Network errors are shown **inline above the buttons** using `Color.info_error(self.error)`.
Not a modal dialog. The error text describes what failed:
```
  Network configuration could not be applied; please verify your settings.

         [  Done / Continue without network  ]
         [  Back  ]
```

The button label changes: if no default route, "Done" → "Continue without network".

#### Install errors (error.py)

Install errors use a `Stretchy` overlay with:
- An intro: "Sorry, there was a problem completing the installation."
- State-dependent body: spinner (collecting) → report view → retry/close buttons
- Per-error-kind action options:
  - `NETWORK_FAIL`: "Continue" (assume no network)
  - `INSTALL_FAIL`: "Restart the installer"
  - `DISK_PROBE_FAIL`: "Switch to a shell" (expert recovery)
  - `SERVER_REQUEST_FAIL`: "Continue" or "Restart"
- Option to send error report to Canonical (`ProgressBar` for upload)
- Option to view full report in `less`

**4 error categories from DESIGN.md:**
1. **Immediate** — shown immediately, installation halted (install fail, autoinstall error)
2. **Delayed** — not shown until critical (block probe errors shown at filesystem screen)
3. **API** — can try something else, not fatal
4. **Client** — UI bug

### Knuckle implications

| Subiquity pattern | Knuckle mapping |
|---|---|
| Validation error in help row below field | `huh` does this natively with `.Error()` |
| Done button disabled while errors exist | `huh` handles — Next is blocked until form valid |
| Character-level rejection with message | Custom `huh` field validator returning error immediately |
| Network error inline above buttons | `lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))` text in view |
| Install error = modal overlay | `huh.Confirm`-style overlay or BubbleTea overlay model |
| Per-error recovery actions | Map error types to specific recovery options |
| "Continue without network" option | Show explicit continue-anyway path for non-fatal errors |
| flatcar-install failure | Error overlay with: log path, "Try again", "Reboot" options |

**Validation rules for knuckle fields** (based on AGENTS.md constraints):
- Hostname: non-empty, ≤ 63 chars, matches `[a-z0-9][a-z0-9-]*` (RFC 952)
- Username: non-empty, ≤ 32 chars, matches `[a-z_][a-z0-9_-]*`
- Password: non-empty (strength check optional)
- Network interface name: 1–15 chars, matches `[a-zA-Z0-9._-]+`
- IP address: valid IPv4 CIDR or single IP
- Gateway: valid IPv4
- Ignition URL: valid `http://` or `https://` URL

---

## 7. Visual Elements

### Color palette

Subiquity uses exactly 8 colors (linux tty constraint via `PIO_CMAP ioctl`).
The palette maps semantic roles to these 8 slots:

| Semantic role | Color name | RGB | urwid name |
|---|---|---|---|
| Background | `bg` | #111111 | black |
| Error/danger | `danger` | #FF0000 | dark red |
| Success/good | `good` | #0E8420 | dark green |
| Brand/header | `orange` | #E95420 | brown |
| Progress/info | `neutral` | #007AA6 | dark blue |
| Unused (brand2) | `brand` | #333333 | dark magenta |
| Secondary/dim | `gray` | #808080 | dark cyan |
| Foreground | `fg` | #FFFFFF | light gray |

**Named styles** (what components actually use):

| Style name | fg | bg | Usage |
|---|---|---|---|
| `frame_header` | fg=white | bg=orange | Header bar text |
| `frame_header_fringe` | orange | bg | Header fringe rows |
| `body` | fg=white | bg=black | Everything else |
| `string_input` | fg=black | bg=white | Text input (inverted!) |
| `string_input focus` | fg=white | bg=gray | Focused text input |
| `done_button focus` | fg=white | bg=green | Done/OK button when focused |
| `danger_button focus` | fg=white | bg=red | Destructive button when focused |
| `other_button focus` | fg=white | bg=gray | Back/Cancel when focused |
| `menu_button focus` | fg=white | bg=gray | List row when selected |
| `info_minor` | fg=gray | bg=black | Dimmed/secondary text |
| `info_error` | fg=red | bg=black | Error messages |
| `progress_complete` | fg=white | bg=blue | Completed progress |
| `progress_incomplete` | fg=white | bg=gray | Incomplete progress |

**Key insight**: All buttons are the SAME unfocused style (white on black), only focus
color differentiates them. Users discover the button type by focusing it.

### Borders and spacing

- **Modal dialogs** (Stretchy): `urwid.LineBox` — `─┐│┘─└│┌` box-drawing characters
- **Progress/log boxes**: `MyLineBox` (custom LineBox with title formatting)
- **No decorative borders elsewhere** — just whitespace for separation
- Column spacing in tables: 2 chars between columns
- Overlay padding: left=3, right=3, top=1, bottom=1 (12 total horizontal chars)
- All overlays max-width = 80 cols → content width ~68 cols

### Form width and centering

```
|←────────── terminal width (e.g. 80) ──────────────→|
|  |←─── center_79: 79% ≈ 63 cols (min 76) ────→|  |
|  |  excerpt text                               |  |
|  |  ┌────────────────────────────────────────┐ |  |
|  |  │ form rows (scrollable ListBox)          │ |  |
|  |  └────────────────────────────────────────┘ |  |
|  |                [ Done ] [ Back ]            |  |
|  |←───────────────────────────────────────────→|  |
```

For narrow forms: `center_63` (63% of terminal width).

### Button format

All buttons use square brackets: `[ label ]`
Forward/menu buttons add a right-pointing triangle: `[ label ▸ ]`
Minimum button width: 14 chars.
Buttons are stacked vertically and centered.

### Focused vs unfocused

The only focus indicator is **background color change**. No cursor glyph, no `>` prefix,
no border highlight. When a widget is focused:
- Text inputs: gray background
- Buttons: semantic color (green/red/gray) background
- List rows: gray background
- Radio/checkbox: the cursor position moves to the widget

### Minimal animation

Subiquity uses animation only where absolutely necessary:
- Spinners only during background operations (network check, disk probe, install)
- Spinner must be explicitly stopped — leaking spinners are a bug
- No decorative animations, no transition effects, no progress bars during config steps

### Typography

Subiquity ships its own console font with extra glyphs.
Key glyphs used:
- `•` (BULLET) in dots spinner: `|•----|`
- `▸` (BLACK RIGHT-POINTING SMALL TRIANGLE) for forward/menu buttons and snap list
- `✓` (CHECK MARK) for verified publishers
- `▄` and `▀` (half-block) for 3D header effect
- `☆` (CIRCLED WHITE STAR) for starred snaps

For knuckle (targeting Flatcar's Linux console), use only characters reliably available
in the VGA/console font. The Flatcar console uses the kernel's default `vga8x16` or
similar. Safe characters: box-drawing `─│┌┐└┘├┤┬┴┼`, `•`, basic arrows `←→↑↓`.

---

## 8. Patterns Directly Adoptable by Knuckle

### Pattern A: The `screen()` layout

Every view should follow this structure:

```
┌─────────────────────────────────── header ──────────────────────────────────────┐
│ ▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄ │
│  Step title here                                                    [?] Help  │
│ ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀ │
└─────────────────────────────────────────────────────────────────────────────────┘

  Brief excerpt: one or two sentences explaining what this step is for.

  ┌ scrollable content area ──────────────────────────────────────────────────┐
  │                                                                           │
  │  [form fields / disk table / selection list / progress log]              │
  │                                                                           │
  └───────────────────────────────────────────────────────────────────────────┘

                          [ Done / Next / Install ]
                          [       Back            ]

```

### Pattern B: Form field with validation

```go
// In huh form:
huh.NewInput().
    Title("Your hostname").
    Description("The name this machine uses on the network").
    Validate(func(s string) error {
        return validate.Hostname(s)
    }).
    Value(&cfg.Hostname)
```

The description appears below the input. On validation failure, huh replaces the
description with the error text in a different style. This matches subiquity exactly.

### Pattern C: Destructive confirmation overlay

```go
// Render as a BubbleTea overlay model
type ConfirmModel struct {
    diskPath string
    focused  int  // 0=cancel, 1=install
}

// Visual:
// ╭──── Confirm Installation ────╮
// │                              │
// │  Erase /dev/disk/by-id/...   │
// │  and install Flatcar Linux?  │
// │                              │
// │  This cannot be undone.      │
// │                              │
// │       [ Cancel  ]            │
// │       [ Install ] ← red      │
// │                              │
// ╰──────────────────────────────╯
```

Always put Cancel before the destructive action. Default focus on Cancel.

### Pattern D: Install progress event log

```go
type ProgressModel struct {
    events    []Event       // log of all events
    active    map[int]bool  // which event IDs have active spinners
    spinner   spinner.Model
    viewport  viewport.Model
    state     AppState
}

// Render each event line:
// "Partitioning disk...   -"  ← spinner if active
// "Formatting /dev/sda1   done" ← plain text when finished
// "Writing Flatcar image... |•---|" ← dots spinner
```

### Pattern E: Inline network error

For non-fatal errors (DHCP timeout, failed validation):
```
  eth0    ethernet   DHCP    not connected
  eth1    ethernet   static  192.168.1.100/24

  Network configuration timed out; please verify your settings.

                [ Continue without network ]
                [          Back           ]
```

Do NOT show a modal for network errors. Show inline text above the buttons.
Let the user continue or go back.

### Pattern F: Background operation indicator

Any operation taking > 100ms should show:
```
╭──────────────────────────────╮
│  Loading sysext catalog  •-- │
│          [ Cancel ]          │
╰──────────────────────────────╯
```

Show for at least 1s to avoid flicker. If the operation supports cancellation, show
a Cancel button. If it can't be cancelled, omit the button but still show the dialog.

---

## 9. Anti-Patterns to Avoid

Based on what subiquity deliberately does NOT do:

| Anti-pattern | Why subiquity avoids it | Knuckle should also avoid |
|---|---|---|
| Sidebar with step list | Wastes 15-20 cols on 80-col terminal | Yes — no sidebar |
| Breadcrumb trail | Same space waste, redundant | Yes — header title is enough |
| Step counter "3 of 8" | Creates artificial anxiety about steps | Yes — omit |
| Toast/snackbar notifications | Not present in urwid ecosystem; instead: inline | Yes |
| Progress bar during config steps | Misleading — steps don't have measurable duration | Yes — spinners only during actual I/O |
| Auto-advance after input | Jarring; user should control advancement | Yes — always require explicit Done |
| Disabled Back button during validation | Users must always be able to go back | Yes — Back ignores validation state |
| Global "spinner overlay" that blocks all input | Only used for very short ops; prefer event log | Use LoadingDialog only for <3s operations |
| Buttons labeled "OK" and "Cancel" | Subiquity uses semantic labels: "Done", "Back", "Continue" | Yes — label buttons with their actual action |
| Putting error text in a modal for field errors | Subiquity always shows field errors inline | Yes — huh handles this correctly |
| "Are you sure?" with "Yes/No" | Subiquity uses "Continue/No" — affirmative maps to action | Use action verbs: "Install/Cancel", not "Yes/No" |

---

## 10. Terminal Sizing Reference

**Minimum supported by subiquity: 80×24**

Screen budget at 80×24:
```
Row 1:  ▄▄▄ header fringe ▄▄▄
Row 2:  Step title           (header)
Row 3:  ▀▀▀ header fringe ▀▀▀
Row 4:  (blank)
Row 5:  Excerpt line 1
Row 6:  Excerpt line 2 (or blank)
Row 7:  (blank)
Rows 8–19: scrollable form content = 12 rows available
Row 20: (blank)
Row 21: [ Done / Next ]
Row 22: [ Back ]
Row 23: (blank)
Row 24: (status/debug line, optional)
```

This gives **12 rows** for scrollable content. The identity form (5 fields × 2 rows each +
1 blank separator × 4 = 14 rows) does NOT fit without scrolling. Subiquity handles this
by making the content area a scrolling `ListBox`. Knuckle must do the same.

**Form field row budget (2 rows per field + 1 separator):**
- 4 fields = 11 rows (fits)
- 5 fields = 14 rows (scrolls)
- 6+ fields = definitely split into multiple steps

**Knuckle step sizing recommendation:**
- Welcome: 1 screen, no form, just excerpt + Done
- Disk selection: scrollable table (may have many disks)
- Network: 3–5 fields (DHCP = 1 toggle; static = 4 fields)
- User config: 3 fields (username, password, confirm) — fits without scroll
- Sysexts: scrollable multi-select list
- Summary/confirm: overlay
- Progress: full screen event log

---

## Key Files Referenced

| File | Purpose |
|---|---|
| `subiquitycore/ui/utils.py` | `screen()` layout helper, `Color`, `Padding`, button styles |
| `subiquitycore/ui/form.py` | Form/field system, validation, two-column layout |
| `subiquitycore/ui/frame.py` | Top-level frame, header management |
| `subiquitycore/ui/anchors.py` | Header widget with half-block fringe |
| `subiquitycore/palette.py` | 8-color palette, all style definitions |
| `subiquitycore/ui/buttons.py` | Button types: done/danger/other/menu/forward |
| `subiquitycore/ui/spinner.py` | Spinner styles: "spin" and "dots" |
| `subiquitycore/ui/stretchy.py` | Modal dialog system (confirmation, errors) |
| `subiquitycore/ui/confirmation.py` | Reusable yes/no confirmation overlay |
| `subiquity/ui/views/identity.py` | User config form — canonical form example |
| `subiquity/ui/views/installprogress.py` | Progress view + confirmation overlay |
| `subiquity/ui/views/filesystem/guided.py` | Disk selection with radio + sub-forms |
| `subiquity/ui/views/snaplist.py` | Multi-select list with checkbox rows |
| `subiquitycore/ui/views/network.py` | Network config with inline error handling |
| `subiquity/ui/views/error.py` | Error overlay with per-kind recovery options |
| `DESIGN.md` | Design principles, 80×24 constraint, async rules |
