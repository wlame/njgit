# Project Instructions for Claude Code

## Git & Version Control Rules

**CRITICAL: NEVER execute `git push` commands**
- User handles all pushes to remote repositories manually
- After committing changes, inform the user they need to push manually
- Allowed git commands: `add`, `commit`, `status`, `diff`, `log`, `show`
- When work is complete, remind user: "Ready to push. Run: `git push origin main`"

## Tool Usage Preferences

### Preferred Tools
- **Read**: For reading files (instead of `cat`)
- **Edit**: For modifying files (instead of `sed`)
- **Write**: For creating files (instead of `echo >`)
- **Bash**: Only for actual shell commands that require it

### Task Agents
- Use Task agent for complex multi-step exploration only
- Don't use Task agent for simple file searches or reads
- Prefer direct Grep/Glob for targeted searches

## Code Quality

### Linting & Testing
- Always run tests after significant changes
- Fix linter issues before committing
- Build and verify binary works after changes

### Commit Messages
- Use conventional commit format when appropriate
- Be descriptive but concise
- No emojis in commit messages

## Communication Style

- Be concise and professional
- No emojis in responses unless explicitly requested
- Provide file paths with line numbers for easy navigation (e.g., `file.go:123`)
- Show progress for multi-step tasks using TodoWrite tool

## Decision Making

- Ask for clarification on ambiguous requirements
- Don't make architectural decisions without user input
- Prefer simple solutions over complex ones
- Keep changes focused on the requested task
