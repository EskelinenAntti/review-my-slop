# PR Review Integration Design

This document outlines the architecture and implementation details for adding GitHub Pull Request review capabilities to `review-my-slop` (`rms`).

## Overview

The goal is to allow users to leave inline comments and code suggestions directly from the `rms` TUI. This requires detecting an active PR context, mapping TUI selections to GitHub diff coordinates, and interacting with the GitHub REST API.

---

## 1. PR Context Detection

Before enabling review features, `rms` must determine if the current working directory is associated with an active GitHub PR.

### Mechanism
Use the GitHub CLI (`gh`) to fetch PR metadata in JSON format.

**Command:**
```bash
gh pr view --json number,headRefOid,headRepository,headRepositoryOwner,baseRefOid \
   --jq '{"number": .number, "owner": .headRepositoryOwner.login, "repo": .headRepository.name, "head": .headRefOid, "base": .baseRefOid}'
```

### Metadata Required
| Field | Purpose |
| :--- | :--- |
| `number` | The PR ID for the API endpoint. |
| `owner` | The repository owner (user or organization). |
| `repo` | The repository name. |
| `head` | The SHA of the PR's head commit (required for comments). |
| `base` | The SHA of the PR's base commit (used to fetch old code). |

---

## 2. User Input & Selection Mode

To support multi-line comments, `rms` implements a "Visual Selection" mode.

### Visual Mode (`v`)
- **Activation**: Pressing `v` toggles selection mode.
- **Anchor**: The line where `v` was pressed becomes the `SelectionAnchor`.
- **Range**: As the cursor moves, the range from `SelectionAnchor` to the current `cursor` is highlighted.
- **Escape**: Pressing `ESC` while selecting clears the selection.
- **Constraints**: 
    - Selections must be within the same file and on the same "side" (all `RIGHT/new` or all `LEFT/old`).
    - **User Experience**: If a move would violate these constraints, the cursor simply does not move.

### Multi-line Suggestions (`s`)
When `s` is pressed:
1. **Source of Truth**: Instead of parsing the TUI display (which may contain line numbers or colors), `rms` fetches the exact code content using `git show <ref>:<path>`.
2. **Ref Selection**: Use the `head` SHA for new/added lines and the `base` SHA for old/deleted lines.
3. **Pre-population**: The block contains the exact source lines:
   ````markdown
   ```suggestion
   [Clean Line 1 content from git]
   [Clean Line 2 content from git]
   ...
   ```
   ````

---

## 3. GitHub API Integration

### Endpoint
`POST /repos/{owner}/{repo}/pulls/{pull_number}/comments`

### Payload Structure
```json
{
  "body": "The comment or suggestion",
  "path": "relative/path/to/file.go",
  "line": 10,
  "side": "RIGHT",
  "start_line": 5,
  "start_side": "RIGHT",
  "commit_id": "HEAD_SHA"
}
```

**Mapping Rules**:
- `new` lines map to `side: "RIGHT"`.
- `old` lines map to `side: "LEFT"`.
- For single-line comments, `start_line` and `start_side` are omitted.

---

## 4. TUI Workflow & UI

### Keybindings
- `v`: **Toggle Selection**. Start or end a multi-line selection.
- `c`: **Comment**. Post a comment to the current line or selected range.
- `s`: **Suggest**. Post a suggestion block to the current line or range.
- `ESC`: **Clear Selection** (if selecting) or **Quit**.
- `j/k`: **Move Cursor** (respecting selection constraints if active).

### State Transitions
1. **Idle**: Single line highlighted.
2. **Selecting**: Range highlighted. `[SELECTING]` indicator in status bar.
3. **Editing/Posting**: TUI enters "Normal" mode (via `stty`) to allow the user's `$EDITOR` to open for writing the comment body.

---

## Knowledge Summary
- **gh api**: Used for all GitHub interactions to leverage local authentication.
- **git show**: Essential for getting "clean" code for suggestions.
- **Side Constraints**: GitHub's multi-line API requires the entire range to be on the same side of the diff.
- **Pathing**: The `path` must be relative to the repository root.
