package review

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// CodeChecker provides language-specific code checking
type CodeChecker struct {
	// Language-specific checkers
	checkers map[string]func(code string) *CheckResult
}

// NewCodeChecker creates a new code checker
func NewCodeChecker() *CodeChecker {
	cc := &CodeChecker{
		checkers: make(map[string]func(code string) *CheckResult),
	}

	// Register Go checker
	cc.checkers["go"] = cc.checkGo
	cc.checkers["golang"] = cc.checkGo

	// Register generic checkers for other languages
	cc.checkers["python"] = cc.checkPython
	cc.checkers["javascript"] = cc.checkJavaScript
	cc.checkers["typescript"] = cc.checkJavaScript
	cc.checkers["js"] = cc.checkJavaScript
	cc.checkers["ts"] = cc.checkJavaScript

	return cc
}

// Check checks code for a given language
func (cc *CodeChecker) Check(language, code string) *CheckResult {
	language = strings.ToLower(language)

	if checker, ok := cc.checkers[language]; ok {
		return checker(code)
	}

	// Unknown language - return passed by default
	return &CheckResult{
		CheckType: CheckTypeSyntax,
		Passed:    true,
		Score:     100,
		Issues:    nil,
	}
}

// checkGo uses go/parser to check Go code syntax
func (cc *CodeChecker) checkGo(code string) *CheckResult {
	fset := token.NewFileSet()

	// Try to parse as a complete file first
	_, err := parser.ParseFile(fset, "code.go", code, parser.AllErrors)
	if err == nil {
		return &CheckResult{
			CheckType: CheckTypeSyntax,
			Passed:    true,
			Score:     100,
			Issues:    nil,
		}
	}

	// If failed, try wrapping in a main package (for snippets)
	wrappedCode := fmt.Sprintf("package main\n\n%s", code)
	_, err2 := parser.ParseFile(fset, "code.go", wrappedCode, parser.AllErrors)
	if err2 == nil {
		return &CheckResult{
			CheckType: CheckTypeSyntax,
			Passed:    true,
			Score:     100,
			Issues:    nil,
		}
	}

	// Try wrapping as function body
	funcWrapped := fmt.Sprintf("package main\nfunc main() {\n%s\n}", code)
	_, err3 := parser.ParseFile(fset, "code.go", funcWrapped, parser.AllErrors)
	if err3 == nil {
		return &CheckResult{
			CheckType: CheckTypeSyntax,
			Passed:    true,
			Score:     100,
			Issues:    nil,
		}
	}

	// Parse the error to get issues
	issues := cc.parseGoErrors(err.Error())

	return &CheckResult{
		CheckType: CheckTypeSyntax,
		Passed:    false,
		Score:     30,
		Issues:    issues,
	}
}

// parseGoErrors extracts error messages from go parser errors
func (cc *CodeChecker) parseGoErrors(errStr string) []string {
	lines := strings.Split(errStr, "\n")
	issues := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Remove file:line:col prefix if present
		if idx := strings.Index(line, ": "); idx != -1 {
			line = line[idx+2:]
		}
		if line != "" {
			issues = append(issues, line)
		}
	}

	// Limit to first 5 issues
	if len(issues) > 5 {
		issues = issues[:5]
		issues = append(issues, fmt.Sprintf("... and %d more issues", len(issues)-5))
	}

	return issues
}

// checkPython does basic Python syntax checking
func (cc *CodeChecker) checkPython(code string) *CheckResult {
	issues := []string{}

	// Check for common Python syntax issues
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for unclosed brackets/parens in single line (simple check)
		if strings.Count(trimmed, "(") != strings.Count(trimmed, ")") ||
			strings.Count(trimmed, "[") != strings.Count(trimmed, "]") ||
			strings.Count(trimmed, "{") != strings.Count(trimmed, "}") {
			// This is a simple heuristic, might have false positives for multiline
			if !strings.HasSuffix(trimmed, "\\") && !strings.HasSuffix(trimmed, ",") {
				issues = append(issues, fmt.Sprintf("Line %d: possible unclosed bracket", i+1))
			}
		}

		// Check for missing colons after def/if/for/while/class/try/except/finally
		keywords := []string{"def ", "if ", "elif ", "else", "for ", "while ", "class ", "try", "except", "finally", "with "}
		for _, kw := range keywords {
			if strings.HasPrefix(trimmed, kw) && !strings.HasSuffix(trimmed, ":") && !strings.HasSuffix(trimmed, "\\") {
				issues = append(issues, fmt.Sprintf("Line %d: missing colon after '%s'", i+1, strings.TrimSpace(kw)))
			}
		}
	}

	if len(issues) > 0 {
		return &CheckResult{
			CheckType: CheckTypeSyntax,
			Passed:    false,
			Score:     50,
			Issues:    issues,
		}
	}

	return &CheckResult{
		CheckType: CheckTypeSyntax,
		Passed:    true,
		Score:     100,
		Issues:    nil,
	}
}

// checkJavaScript does basic JavaScript/TypeScript syntax checking
func (cc *CodeChecker) checkJavaScript(code string) *CheckResult {
	issues := []string{}

	// Check for unclosed braces/brackets (simple check)
	braceCount := strings.Count(code, "{") - strings.Count(code, "}")
	bracketCount := strings.Count(code, "[") - strings.Count(code, "]")
	parenCount := strings.Count(code, "(") - strings.Count(code, ")")

	if braceCount != 0 {
		issues = append(issues, fmt.Sprintf("Unbalanced braces: %+d", braceCount))
	}
	if bracketCount != 0 {
		issues = append(issues, fmt.Sprintf("Unbalanced brackets: %+d", bracketCount))
	}
	if parenCount != 0 {
		issues = append(issues, fmt.Sprintf("Unbalanced parentheses: %+d", parenCount))
	}

	// Check for common issues
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for var usage (suggest let/const)
		if strings.Contains(trimmed, "var ") {
			issues = append(issues, fmt.Sprintf("Line %d: consider using 'let' or 'const' instead of 'var'", i+1))
		}

		// Check for == instead of ===
		eqPattern := regexp.MustCompile(`[^=!]==[^=]`)
		if eqPattern.MatchString(trimmed) {
			issues = append(issues, fmt.Sprintf("Line %d: consider using '===' instead of '=='", i+1))
		}
	}

	if len(issues) > 0 && (braceCount != 0 || bracketCount != 0 || parenCount != 0) {
		return &CheckResult{
			CheckType: CheckTypeSyntax,
			Passed:    false,
			Score:     50,
			Issues:    issues,
		}
	}

	// Style issues don't fail the check
	score := 100 - len(issues)*5
	if score < 70 {
		score = 70
	}

	return &CheckResult{
		CheckType:   CheckTypeSyntax,
		Passed:      true,
		Score:       score,
		Issues:      nil,
		Suggestions: issues,
	}
}

// CreateSyntaxCheckStage creates a pipeline stage for syntax checking
func CreateSyntaxCheckStage() PipelineStage {
	checker := NewCodeChecker()

	return PipelineStage{
		Name:      "Syntax Check",
		CheckType: CheckTypeSyntax,
		Required:  true,
		Check: func(ctx context.Context, code string) (*CheckResult, error) {
			// Extract code blocks and check each one
			blocks := ExtractCodeBlocks(code)

			if len(blocks) == 0 {
				// No code blocks found
				return &CheckResult{
					CheckType: CheckTypeSyntax,
					Passed:    true,
					Score:     100,
					Issues:    nil,
				}, nil
			}

			var allIssues []string
			totalScore := 0
			passed := true

			for _, block := range blocks {
				result := checker.Check(block.Language, block.Content)
				totalScore += result.Score
				if !result.Passed {
					passed = false
					for _, issue := range result.Issues {
						allIssues = append(allIssues, fmt.Sprintf("[%s] %s", block.Language, issue))
					}
				}
			}

			avgScore := totalScore / len(blocks)

			return &CheckResult{
				CheckType: CheckTypeSyntax,
				Passed:    passed,
				Score:     avgScore,
				Issues:    allIssues,
			}, nil
		},
	}
}
