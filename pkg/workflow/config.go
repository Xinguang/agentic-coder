package workflow

// WorkflowConfig holds workflow configuration
type WorkflowConfig struct {
	// Concurrency settings
	MaxExecutors int `yaml:"max_executors" json:"max_executors"`
	MaxReviewers int `yaml:"max_reviewers" json:"max_reviewers"`
	MaxFixers    int `yaml:"max_fixers" json:"max_fixers"`

	// Retry settings
	MaxRetries    int  `yaml:"max_retries" json:"max_retries"`
	EnableAutoFix bool `yaml:"enable_auto_fix" json:"enable_auto_fix"`

	// Model settings
	Models RoleModels `yaml:"models" json:"models"`
}

// RoleModels configures AI models for each role
type RoleModels struct {
	Default   string `yaml:"default" json:"default"`
	Manager   string `yaml:"manager,omitempty" json:"manager,omitempty"`
	Executor  string `yaml:"executor,omitempty" json:"executor,omitempty"`
	Reviewer  string `yaml:"reviewer,omitempty" json:"reviewer,omitempty"`
	Fixer     string `yaml:"fixer,omitempty" json:"fixer,omitempty"`
	Evaluator string `yaml:"evaluator,omitempty" json:"evaluator,omitempty"`
}

// DefaultConfig returns the default workflow configuration
func DefaultConfig() *WorkflowConfig {
	return &WorkflowConfig{
		MaxExecutors:  5,
		MaxReviewers:  2,
		MaxFixers:     2,
		MaxRetries:    3,
		EnableAutoFix: true,
		Models: RoleModels{
			Default: "sonnet",
		},
	}
}

// GetModel returns the model for a role, falling back to default
func (m *RoleModels) GetModel(role Role) string {
	switch role {
	case RoleManager:
		if m.Manager != "" {
			return m.Manager
		}
	case RoleExecutor:
		if m.Executor != "" {
			return m.Executor
		}
	case RoleReviewer:
		if m.Reviewer != "" {
			return m.Reviewer
		}
	case RoleFixer:
		if m.Fixer != "" {
			return m.Fixer
		}
	case RoleEvaluator:
		if m.Evaluator != "" {
			return m.Evaluator
		}
	}

	if m.Default != "" {
		return m.Default
	}
	return "sonnet"
}

// Validate validates the configuration
func (c *WorkflowConfig) Validate() error {
	if c.MaxExecutors <= 0 {
		c.MaxExecutors = 5
	}
	if c.MaxReviewers <= 0 {
		c.MaxReviewers = 2
	}
	if c.MaxFixers <= 0 {
		c.MaxFixers = 2
	}
	if c.MaxRetries < 0 {
		c.MaxRetries = 3
	}
	if c.Models.Default == "" {
		c.Models.Default = "sonnet"
	}
	return nil
}
