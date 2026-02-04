// Package skill provides slash command and skill management
package skill

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Skill represents a user-invocable skill or slash command
type Skill struct {
	// Metadata from YAML frontmatter
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Aliases     []string `yaml:"aliases,omitempty" json:"aliases,omitempty"`
	Args        []Arg    `yaml:"args,omitempty" json:"args,omitempty"`
	Location    string   `yaml:"-" json:"location"` // file, plugin, builtin

	// Content
	Content    string `yaml:"-" json:"content"`     // The skill prompt content
	SourceFile string `yaml:"-" json:"source_file"` // Source file path
	PluginName string `yaml:"-" json:"plugin_name"` // Plugin name if from plugin
}

// Arg represents a skill argument
type Arg struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Default     string `yaml:"default,omitempty" json:"default,omitempty"`
	Type        string `yaml:"type,omitempty" json:"type,omitempty"` // string, file, directory
}

// Registry manages skill registration and lookup
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*Skill
}

// Manager is an alias for Registry for external use
type Manager = Registry

// NewRegistry creates a new skill registry
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*Skill),
	}
}

// NewManager creates a new skill manager (alias for NewRegistry)
func NewManager() *Manager {
	return NewRegistry()
}

// Register adds a skill to the registry
func (r *Registry) Register(skill *Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}

	// Register main name
	r.skills[skill.Name] = skill

	// Register aliases
	for _, alias := range skill.Aliases {
		r.skills[alias] = skill
	}

	return nil
}

// Get retrieves a skill by name or alias (returns nil if not found)
func (r *Registry) Get(name string) *Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Remove leading slash if present
	name = strings.TrimPrefix(name, "/")

	if skill, ok := r.skills[name]; ok {
		return skill
	}

	return nil
}

// GetWithError retrieves a skill by name or alias with error
func (r *Registry) GetWithError(name string) (*Skill, error) {
	skill := r.Get(name)
	if skill == nil {
		return nil, fmt.Errorf("skill not found: %s", name)
	}
	return skill, nil
}

// List returns all registered skills (deduplicated)
func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	var skills []*Skill

	for _, skill := range r.skills {
		if !seen[skill.Name] {
			seen[skill.Name] = true
			skills = append(skills, skill)
		}
	}

	return skills
}

// Names returns all skill names (including aliases)
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	return names
}

// Execute executes a skill by name with arguments
func (r *Registry) Execute(ctx context.Context, name string, argString string) (string, error) {
	skill := r.Get(name)
	if skill == nil {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	// Parse arguments
	args, err := ParseArgs(argString, skill.Args)
	if err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Execute skill
	result, err := skill.Execute(args, nil)
	if err != nil {
		return "", fmt.Errorf("skill execution failed: %w", err)
	}

	return result, nil
}

// LoadFromDirectory loads skills from a directory
func (r *Registry) LoadFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".md" && ext != ".txt" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		skill, err := LoadSkillFromFile(path)
		if err != nil {
			// Skip files that can't be parsed as skills
			continue
		}

		skill.Location = "file"
		if err := r.Register(skill); err != nil {
			continue
		}
	}

	return nil
}

// LoadSkillFromFile loads a skill from a file
func LoadSkillFromFile(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	skill, err := ParseSkill(string(content))
	if err != nil {
		return nil, err
	}

	skill.SourceFile = path

	// Use filename as name if not specified
	if skill.Name == "" {
		baseName := filepath.Base(path)
		skill.Name = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}

	return skill, nil
}

// ParseSkill parses a skill from content with YAML frontmatter
func ParseSkill(content string) (*Skill, error) {
	skill := &Skill{}

	// Check for YAML frontmatter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			// Parse YAML frontmatter
			if err := yaml.Unmarshal([]byte(parts[1]), skill); err != nil {
				return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
			}
			skill.Content = strings.TrimSpace(parts[2])
		} else {
			skill.Content = content
		}
	} else {
		skill.Content = content
	}

	return skill, nil
}

// Execute generates the expanded prompt for a skill
func (s *Skill) Execute(args map[string]string, context *ExecutionContext) (string, error) {
	content := s.Content

	// Replace argument placeholders
	for _, arg := range s.Args {
		placeholder := fmt.Sprintf("{{%s}}", arg.Name)
		value := args[arg.Name]

		if value == "" && arg.Default != "" {
			value = arg.Default
		}

		if value == "" && arg.Required {
			return "", fmt.Errorf("required argument missing: %s", arg.Name)
		}

		content = strings.ReplaceAll(content, placeholder, value)
	}

	// Replace context placeholders
	if context != nil {
		content = strings.ReplaceAll(content, "{{cwd}}", context.CWD)
		content = strings.ReplaceAll(content, "{{project_path}}", context.ProjectPath)
		content = strings.ReplaceAll(content, "{{session_id}}", context.SessionID)
	}

	// Handle special placeholders
	content = expandSpecialPlaceholders(content)

	return content, nil
}

// ExecutionContext provides context for skill execution
type ExecutionContext struct {
	SessionID   string
	ProjectPath string
	CWD         string
}

// expandSpecialPlaceholders expands special placeholders
func expandSpecialPlaceholders(content string) string {
	// {{date}} - current date
	// {{time}} - current time
	// {{env:VAR}} - environment variable
	// {{file:path}} - file content

	// Environment variables
	envRegex := regexp.MustCompile(`\{\{env:([^}]+)\}\}`)
	content = envRegex.ReplaceAllStringFunc(content, func(match string) string {
		varName := envRegex.FindStringSubmatch(match)[1]
		return os.Getenv(varName)
	})

	// File content (limited to prevent abuse)
	fileRegex := regexp.MustCompile(`\{\{file:([^}]+)\}\}`)
	content = fileRegex.ReplaceAllStringFunc(content, func(match string) string {
		filePath := fileRegex.FindStringSubmatch(match)[1]
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Sprintf("[Error reading file: %s]", err)
		}
		// Limit file content
		if len(data) > 10000 {
			return string(data[:10000]) + "\n[... truncated ...]"
		}
		return string(data)
	})

	return content
}

// ParseArgs parses command-line style arguments
func ParseArgs(argString string, argDefs []Arg) (map[string]string, error) {
	result := make(map[string]string)

	if argString == "" {
		return result, nil
	}

	// Simple argument parsing
	scanner := bufio.NewScanner(strings.NewReader(argString))
	scanner.Split(bufio.ScanWords)

	positionalIndex := 0
	for scanner.Scan() {
		token := scanner.Text()

		// Named argument: --name=value or --name value
		if strings.HasPrefix(token, "--") {
			parts := strings.SplitN(token[2:], "=", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			} else if scanner.Scan() {
				result[parts[0]] = scanner.Text()
			}
			continue
		}

		// Short argument: -n value
		if strings.HasPrefix(token, "-") && len(token) == 2 {
			if scanner.Scan() {
				// Find arg by first letter
				argName := string(token[1])
				for _, arg := range argDefs {
					if strings.HasPrefix(arg.Name, argName) {
						result[arg.Name] = scanner.Text()
						break
					}
				}
			}
			continue
		}

		// Positional argument
		if positionalIndex < len(argDefs) {
			result[argDefs[positionalIndex].Name] = token
			positionalIndex++
		}
	}

	return result, nil
}

// BuiltinSkills returns the built-in skills
func BuiltinSkills() []*Skill {
	return []*Skill{
		{
			Name:        "commit",
			Description: "Create a git commit with AI-generated message",
			Location:    "builtin",
			Content: `Review all staged changes and create a commit.

1. Run git status and git diff --staged to see changes
2. Analyze the changes and generate a concise commit message
3. Follow conventional commit format: type: description
4. Create the commit

Keep the message under 72 characters. Focus on the "why" not the "what".`,
		},
		{
			Name:        "review",
			Description: "Review code changes in the current branch",
			Aliases:     []string{"review-pr"},
			Location:    "builtin",
			Args: []Arg{
				{Name: "branch", Description: "Branch to compare against", Default: "main"},
			},
			Content: `Review code changes compared to {{branch}} branch.

1. Get the diff: git diff {{branch}}...HEAD
2. Analyze the changes for:
   - Code quality issues
   - Potential bugs
   - Security concerns
   - Performance implications
   - Missing tests
3. Provide constructive feedback with specific suggestions`,
		},
		{
			Name:        "test",
			Description: "Run tests and fix failures",
			Location:    "builtin",
			Content: `Run the test suite and fix any failing tests.

1. Detect the test framework (go test, npm test, pytest, etc.)
2. Run the tests
3. If tests fail, analyze the failures
4. Fix the issues and re-run until all tests pass`,
		},
		{
			Name:        "explain",
			Description: "Explain how code works",
			Location:    "builtin",
			Args: []Arg{
				{Name: "target", Description: "File or function to explain", Required: true},
			},
			Content: `Explain how {{target}} works.

1. Read the relevant code
2. Explain the purpose and functionality
3. Describe the key components and their interactions
4. Note any important patterns or design decisions`,
		},
		{
			Name:        "fix",
			Description: "Fix a bug or issue",
			Location:    "builtin",
			Args: []Arg{
				{Name: "issue", Description: "Description of the issue", Required: true},
			},
			Content: `Fix the following issue: {{issue}}

1. Understand the problem
2. Search for relevant code
3. Identify the root cause
4. Implement the fix
5. Verify the fix works`,
		},
		{
			Name:        "refactor",
			Description: "Refactor code for better quality",
			Location:    "builtin",
			Args: []Arg{
				{Name: "target", Description: "File or function to refactor", Required: true},
			},
			Content: `Refactor {{target}} for better code quality.

1. Read and understand the current implementation
2. Identify areas for improvement
3. Refactor while maintaining functionality
4. Ensure tests still pass`,
		},
		{
			Name:        "doc",
			Description: "Generate documentation",
			Location:    "builtin",
			Args: []Arg{
				{Name: "target", Description: "File or function to document"},
			},
			Content: `Generate documentation for {{target}}.

1. Read the code
2. Generate clear, helpful documentation
3. Include examples where appropriate
4. Follow the project's documentation style`,
		},
	}
}
