package review

import (
	"testing"
)

func TestCodeChecker_Go_ValidCode(t *testing.T) {
	checker := NewCodeChecker()

	tests := []struct {
		name string
		code string
	}{
		{
			name: "complete file",
			code: `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}`,
		},
		{
			name: "function only",
			code: `func add(a, b int) int {
	return a + b
}`,
		},
		{
			name: "statements only",
			code: `x := 1
y := 2
z := x + y`,
		},
		{
			name: "struct definition",
			code: `type Person struct {
	Name string
	Age  int
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.Check("go", tt.code)
			if !result.Passed {
				t.Errorf("expected valid Go code to pass, got issues: %v", result.Issues)
			}
		})
	}
}

func TestCodeChecker_Go_InvalidCode(t *testing.T) {
	checker := NewCodeChecker()

	tests := []struct {
		name string
		code string
	}{
		{
			name: "missing closing brace",
			code: `func main() {
	fmt.Println("Hello")`,
		},
		{
			name: "syntax error",
			code: `func main() {
	x := := 1
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.Check("go", tt.code)
			if result.Passed {
				t.Error("expected invalid Go code to fail")
			}
			if len(result.Issues) == 0 {
				t.Error("expected issues to be reported")
			}
		})
	}
}

func TestCodeChecker_JavaScript(t *testing.T) {
	checker := NewCodeChecker()

	tests := []struct {
		name       string
		code       string
		wantPassed bool
	}{
		{
			name:       "valid code",
			code:       `const x = 1;\nconst y = 2;`,
			wantPassed: true,
		},
		{
			name:       "unbalanced braces",
			code:       `function test() {\n  console.log("hi")`,
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.Check("javascript", tt.code)
			if result.Passed != tt.wantPassed {
				t.Errorf("expected Passed=%v, got %v", tt.wantPassed, result.Passed)
			}
		})
	}
}

func TestCodeChecker_Python(t *testing.T) {
	checker := NewCodeChecker()

	tests := []struct {
		name       string
		code       string
		wantPassed bool
	}{
		{
			name:       "valid code",
			code:       "def hello():\n    print('hello')",
			wantPassed: true,
		},
		{
			name:       "missing colon",
			code:       "def hello()\n    print('hello')",
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.Check("python", tt.code)
			if result.Passed != tt.wantPassed {
				t.Errorf("expected Passed=%v, got %v (issues: %v)", tt.wantPassed, result.Passed, result.Issues)
			}
		})
	}
}

func TestCodeChecker_UnknownLanguage(t *testing.T) {
	checker := NewCodeChecker()

	result := checker.Check("unknown", "some code")
	if !result.Passed {
		t.Error("unknown language should pass by default")
	}
	if result.Score != 100 {
		t.Errorf("expected score 100, got %d", result.Score)
	}
}

func TestCreateSyntaxCheckStage(t *testing.T) {
	stage := CreateSyntaxCheckStage()

	if stage.Name != "Syntax Check" {
		t.Errorf("expected name 'Syntax Check', got %q", stage.Name)
	}

	if !stage.Required {
		t.Error("syntax check should be required")
	}

	// Test with valid Go code block
	code := "```go\nfunc main() {}\n```"
	result, err := stage.Check(nil, code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Passed {
		t.Errorf("valid code should pass, got issues: %v", result.Issues)
	}
}

func TestCreateSyntaxCheckStage_InvalidCode(t *testing.T) {
	stage := CreateSyntaxCheckStage()

	// Test with invalid Go code block
	code := "```go\nfunc main() {\n```"
	result, err := stage.Check(nil, code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("invalid code should fail")
	}
}

func TestCreateSyntaxCheckStage_NoCodeBlocks(t *testing.T) {
	stage := CreateSyntaxCheckStage()

	// Test with no code blocks
	code := "This is just text without any code blocks."
	result, err := stage.Check(nil, code)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Passed {
		t.Error("no code blocks should pass")
	}
}
