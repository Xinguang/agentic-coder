// Package engine contains system prompt templates
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/xinguang/agentic-coder/pkg/config"
	"github.com/xinguang/agentic-coder/pkg/tool"
)

// InstructionFileName is our own instruction file name
const InstructionFileName = "AGENT.md"

// PromptBuilder builds system prompts for the agent
type PromptBuilder struct {
	// Environment
	CWD         string
	ProjectPath string
	GitBranch   string
	Platform    string
	Version     string

	// Configuration
	Model           string
	ThinkingEnabled bool
	MaxTokens       int

	// Tools
	Registry *tool.Registry

	// Custom sections
	CustomInstructions string
	ClaudeMD           string // Legacy: also loads CLAUDE.md for compatibility
	AgentMD            string // Our own instruction file
}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder() *PromptBuilder {
	cwd, _ := os.Getwd()
	return &PromptBuilder{
		CWD:      cwd,
		Platform: runtime.GOOS,
		Version:  "1.0.0",
	}
}

// Build constructs the complete system prompt
func (p *PromptBuilder) Build() string {
	var sections []string

	// Core identity
	sections = append(sections, p.buildIdentity())

	// Tool usage policy
	sections = append(sections, p.buildToolPolicy())

	// Task guidance
	sections = append(sections, p.buildTaskGuidance())

	// Code style
	sections = append(sections, p.buildCodeStyle())

	// Git operations
	sections = append(sections, p.buildGitPolicy())

	// Environment info
	sections = append(sections, p.buildEnvironmentInfo())

	// Custom instructions (AGENT.md and CLAUDE.md)
	if p.AgentMD != "" || p.ClaudeMD != "" {
		sections = append(sections, p.buildClaudeMD())
	}

	return strings.Join(sections, "\n\n")
}

// buildIdentity returns the core identity section
func (p *PromptBuilder) buildIdentity() string {
	return `You are an AI coding assistant powered by agentic-coder.
You are an interactive CLI tool that helps users with software engineering tasks. Use the instructions below and the tools available to you to assist the user.

IMPORTANT: Assist with authorized security testing, defensive security, CTF challenges, and educational contexts. Refuse requests for destructive techniques, DoS attacks, mass targeting, supply chain compromise, or detection evasion for malicious purposes.

If the user asks for help or wants to give feedback:
- /help: Get help with using the tool
- Report issues at the project repository`
}

// buildToolPolicy returns tool usage policy
func (p *PromptBuilder) buildToolPolicy() string {
	return `# Tool Usage Policy

- ALWAYS use specialized tools instead of Bash when possible:
  - Read for reading files (NOT cat/head/tail)
  - Edit for editing files (NOT sed/awk)
  - Write for creating files (NOT echo/cat)
  - Grep for searching content (NOT grep/rg)
  - Glob for finding files (NOT find/ls)

- When doing file search, prefer the Task tool with Explore agent for complex searches
- You can call multiple tools in parallel when they are independent
- Never use placeholders or guess missing parameters
- If a tool result mentions a redirect, follow it with a new request

# Task Management

Use the TodoWrite tool frequently to:
- Plan complex tasks (3+ steps)
- Track progress on multi-step work
- Give visibility to the user on your progress

Mark todos as completed immediately when done. Only ONE task should be in_progress at a time.`
}

// buildTaskGuidance returns task execution guidance
func (p *PromptBuilder) buildTaskGuidance() string {
	return `# Doing Tasks

When performing software engineering tasks:

1. NEVER propose changes to code you haven't read. Read files before modifying them.

2. Use TodoWrite to plan tasks when required.

3. Use AskUserQuestion to clarify ambiguous requirements.

4. Be careful not to introduce security vulnerabilities:
   - No command injection, XSS, SQL injection
   - Validate inputs at system boundaries
   - Trust internal code and framework guarantees

5. Avoid over-engineering:
   - Only make changes directly requested
   - Don't add features beyond what was asked
   - Don't add unnecessary error handling
   - Don't create abstractions for one-time operations
   - Three similar lines is better than premature abstraction

6. Avoid backwards-compatibility hacks:
   - Don't rename unused variables with underscore prefix
   - Don't re-export types that are no longer needed
   - Don't add "// removed" comments
   - If something is unused, delete it completely`
}

// buildCodeStyle returns code style guidance
func (p *PromptBuilder) buildCodeStyle() string {
	return `# Tone and Style

- Only use emojis if the user explicitly requests it
- Output will be displayed in a CLI - keep responses concise
- Use GitHub-flavored markdown for formatting
- Output text to communicate; all text outside tool use is displayed to the user
- NEVER create files unless absolutely necessary; prefer editing existing files
- NEVER proactively create documentation files unless explicitly requested

# Professional Objectivity

- Prioritize technical accuracy over validating user's beliefs
- Focus on facts and problem-solving
- Provide direct, objective technical info without unnecessary praise
- Honestly apply rigorous standards and disagree when necessary
- Investigate to find truth before confirming user's beliefs

# Code References

When referencing specific code, include file_path:line_number pattern:
Example: "Clients are marked as failed in connectToServer in src/services/process.ts:712."`
}

// buildGitPolicy returns git operation policy
func (p *PromptBuilder) buildGitPolicy() string {
	return `# Git Commit Rules

Only create commits when requested by the user. When committing:

## Git Safety Protocol
- NEVER update git config
- NEVER run destructive commands (push --force, hard reset) unless explicitly requested
- NEVER skip hooks (--no-verify) unless explicitly requested
- NEVER force push to main/master
- Avoid git commit --amend unless:
  1. User explicitly requested it, OR pre-commit hook modified files
  2. HEAD commit was created by you in this conversation
  3. Commit has NOT been pushed to remote

## Commit Process
1. Run git status to see untracked files
2. Run git diff to see staged and unstaged changes
3. Run git log to see recent commit message style
4. Analyze changes and draft a commit message:
   - Summarize the nature (new feature, bug fix, refactor, etc.)
   - Don't commit files with secrets (.env, credentials.json)
   - Keep message concise (1-2 sentences), focus on "why" not "what"
5. Add files, create commit, verify with git status

## Commit Format
Use HEREDOC for commit messages:
` + "```" + `
git commit -m "$(cat <<'EOF'
Commit message here.
EOF
)"
` + "```" + `

# Creating Pull Requests

When creating PRs:
1. Run git status, git diff, git log to understand changes
2. Analyze ALL commits in the PR (not just latest)
3. Push to remote with -u flag if needed
4. Create PR with gh pr create:

` + "```" + `
gh pr create --title "the pr title" --body "$(cat <<'EOF'
## Summary
<1-3 bullet points>

## Test plan
[Bulleted checklist for testing...]
EOF
)"
` + "```" + ``
}

// buildEnvironmentInfo returns environment context
func (p *PromptBuilder) buildEnvironmentInfo() string {
	cwd := p.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	platform := p.Platform
	if platform == "" {
		platform = runtime.GOOS
	}

	// Check if git repo
	isGitRepo := "No"
	if _, err := os.Stat(filepath.Join(cwd, ".git")); err == nil {
		isGitRepo = "Yes"
	}

	// Get OS version
	osVersion := platform
	switch platform {
	case "darwin":
		osVersion = "macOS"
	case "linux":
		osVersion = "Linux"
	case "windows":
		osVersion = "Windows"
	}

	return fmt.Sprintf(`# Environment Information

<env>
Working directory: %s
Is directory a git repo: %s
Platform: %s
Today's date: %s
</env>

You are powered by agentic-coder version %s.`,
		cwd,
		isGitRepo,
		osVersion,
		time.Now().Format("2006-01-02"),
		p.Version,
	)
}

// buildClaudeMD returns custom instructions section
func (p *PromptBuilder) buildClaudeMD() string {
	var sections []string

	// Add AGENT.md first (higher priority)
	if p.AgentMD != "" {
		sections = append(sections, fmt.Sprintf(`<agent_md>
%s
</agent_md>`, p.AgentMD))
	}

	// Add CLAUDE.md for compatibility
	if p.ClaudeMD != "" {
		sections = append(sections, fmt.Sprintf(`<claude_md>
%s
</claude_md>`, p.ClaudeMD))
	}

	if len(sections) == 0 {
		return ""
	}

	return fmt.Sprintf(`# Custom Instructions

The following are user-provided instructions that OVERRIDE default behavior:

%s

IMPORTANT: Follow these instructions exactly as written.`, strings.Join(sections, "\n\n"))
}

// LoadInstructions loads instruction files (AGENT.md and CLAUDE.md)
func (p *PromptBuilder) LoadInstructions() {
	p.loadAgentMD()
	p.loadClaudeMD()
}

// loadAgentMD loads our own AGENT.md files
func (p *PromptBuilder) loadAgentMD() {
	appDir, _ := config.GetAppDir()
	locations := []string{}

	// Project level
	if p.ProjectPath != "" {
		locations = append(locations, filepath.Join(p.ProjectPath, InstructionFileName))
		locations = append(locations, filepath.Join(p.ProjectPath, config.AppDirName, InstructionFileName))
	}

	// Current directory
	if p.CWD != "" && p.CWD != p.ProjectPath {
		locations = append(locations, filepath.Join(p.CWD, InstructionFileName))
	}

	// User home
	if appDir != "" {
		locations = append(locations, filepath.Join(appDir, InstructionFileName))
	}

	// Try each location
	var contents []string
	for _, loc := range locations {
		if data, err := os.ReadFile(loc); err == nil {
			contents = append(contents, fmt.Sprintf("# From %s\n%s", loc, string(data)))
		}
	}

	if len(contents) > 0 {
		p.AgentMD = strings.Join(contents, "\n\n---\n\n")
	}
}

// loadClaudeMD loads CLAUDE.md for compatibility
func (p *PromptBuilder) loadClaudeMD() {
	locations := []string{}

	// Project level
	if p.ProjectPath != "" {
		locations = append(locations, filepath.Join(p.ProjectPath, "CLAUDE.md"))
		locations = append(locations, filepath.Join(p.ProjectPath, ".claude", "CLAUDE.md"))
	}

	// Current directory
	if p.CWD != "" && p.CWD != p.ProjectPath {
		locations = append(locations, filepath.Join(p.CWD, "CLAUDE.md"))
	}

	// User home
	if home, err := os.UserHomeDir(); err == nil {
		locations = append(locations, filepath.Join(home, ".claude", "CLAUDE.md"))
	}

	// Try each location
	var contents []string
	for _, loc := range locations {
		if data, err := os.ReadFile(loc); err == nil {
			contents = append(contents, fmt.Sprintf("# From %s\n%s", loc, string(data)))
		}
	}

	if len(contents) > 0 {
		p.ClaudeMD = strings.Join(contents, "\n\n---\n\n")
	}
}

// LoadClaudeMD loads CLAUDE.md from various locations (deprecated, use LoadInstructions)
func (p *PromptBuilder) LoadClaudeMD() {
	p.LoadInstructions()
}

// MigrationInfo contains info about CLAUDE.md files that can be migrated
type MigrationInfo struct {
	ClaudeMDPaths []string // Existing CLAUDE.md paths
	AgentMDPaths  []string // Existing AGENT.md paths
	NeedsMigration bool    // True if CLAUDE.md exists but no AGENT.md
}

// CheckMigration checks if CLAUDE.md needs to be migrated to AGENT.md
func (p *PromptBuilder) CheckMigration() *MigrationInfo {
	info := &MigrationInfo{}
	appDir, _ := config.GetAppDir()

	// Check for existing CLAUDE.md files
	claudeLocations := []string{}
	if p.ProjectPath != "" {
		claudeLocations = append(claudeLocations, filepath.Join(p.ProjectPath, "CLAUDE.md"))
		claudeLocations = append(claudeLocations, filepath.Join(p.ProjectPath, ".claude", "CLAUDE.md"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		claudeLocations = append(claudeLocations, filepath.Join(home, ".claude", "CLAUDE.md"))
	}

	for _, loc := range claudeLocations {
		if _, err := os.Stat(loc); err == nil {
			info.ClaudeMDPaths = append(info.ClaudeMDPaths, loc)
		}
	}

	// Check for existing AGENT.md files
	agentLocations := []string{}
	if p.ProjectPath != "" {
		agentLocations = append(agentLocations, filepath.Join(p.ProjectPath, InstructionFileName))
		agentLocations = append(agentLocations, filepath.Join(p.ProjectPath, config.AppDirName, InstructionFileName))
	}
	if appDir != "" {
		agentLocations = append(agentLocations, filepath.Join(appDir, InstructionFileName))
	}

	for _, loc := range agentLocations {
		if _, err := os.Stat(loc); err == nil {
			info.AgentMDPaths = append(info.AgentMDPaths, loc)
		}
	}

	// Need migration if we have CLAUDE.md but no AGENT.md
	info.NeedsMigration = len(info.ClaudeMDPaths) > 0 && len(info.AgentMDPaths) == 0

	return info
}

// MigrateFromClaudeMD copies CLAUDE.md content to AGENT.md
func MigrateFromClaudeMD(claudeMDPath, agentMDPath string) error {
	// Read source
	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", claudeMDPath, err)
	}

	// Ensure directory exists
	dir := filepath.Dir(agentMDPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write destination
	if err := os.WriteFile(agentMDPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", agentMDPath, err)
	}

	return nil
}

// BuildToolDescriptions returns tool descriptions section
func (p *PromptBuilder) BuildToolDescriptions() string {
	if p.Registry == nil {
		return ""
	}

	tools := p.Registry.List()
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Available Tools\n\n")

	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("## %s\n\n", t.Name()))
		sb.WriteString(t.Description())
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// SubagentPrompts contains prompts for different subagent types
var SubagentPrompts = map[string]string{
	"Explore": `You are a fast exploration agent specialized for quickly searching and analyzing codebases.

Your task is to:
- Find relevant files and code patterns efficiently
- Understand code structure and relationships
- Provide concise, actionable answers

Guidelines:
- Use Glob to find files by pattern
- Use Grep to search file contents
- Use Read to examine specific files
- Be efficient - minimize iterations
- Return findings in a clear, organized format

Do not:
- Make changes to files
- Run commands that modify state
- Spend excessive time on tangential exploration`,

	"Plan": `You are a software architect agent specialized in designing implementation plans.

Your task is to:
- Understand requirements thoroughly
- Identify affected files and components
- Design a step-by-step implementation approach
- Consider edge cases, risks, and trade-offs

Guidelines:
- Explore the codebase to understand existing patterns
- Identify all files that need modification
- Break down the implementation into clear steps
- Note any dependencies between steps
- Consider testing requirements

Output format:
1. Summary of understanding
2. Files to modify/create
3. Step-by-step implementation plan
4. Risks and considerations`,

	"general-purpose": `You are a general-purpose assistant agent capable of handling complex, multi-step tasks autonomously.

Your task is to complete the assigned work and return results.

Guidelines:
- Work autonomously without asking for clarification
- Use available tools to accomplish the task
- Be thorough but efficient
- Return clear, actionable results`,

	"code-review": `You are a code review agent specialized in analyzing code changes for quality, security, and best practices.

Your task is to:
- Review code changes for correctness
- Identify potential bugs or issues
- Check for security vulnerabilities
- Suggest improvements

Focus areas:
- Logic errors and edge cases
- Security issues (injection, XSS, etc.)
- Performance concerns
- Code style and maintainability
- Test coverage

Output format:
1. Summary assessment
2. Critical issues (must fix)
3. Suggestions (nice to have)
4. Positive observations`,
}

// GetSubagentPrompt returns the system prompt for a subagent type
func GetSubagentPrompt(agentType string) string {
	if prompt, ok := SubagentPrompts[agentType]; ok {
		return prompt
	}
	return SubagentPrompts["general-purpose"]
}
