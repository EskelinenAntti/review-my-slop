# Repository Instructions

- Run one shell command at a time. Do not chain commands with `&&`, `||`, `;`, pipes, or similar shell operators.
- Do not override `GOCACHE` or other Go environment settings to work around sandbox or permission failures.
- If a standard Go command is blocked by sandbox permissions, immediately retry the same command with an approval request instead of changing the command or Go environment.
