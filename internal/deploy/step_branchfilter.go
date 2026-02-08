package deploy

import (
	"fmt"
	"regexp"
)

type BranchFilterStep struct{}

func (s *BranchFilterStep) Name() string { return "branch-filter" }

func (s *BranchFilterStep) Run(ctx *StepContext) error {
	if ctx.ReviewConfig == nil {
		return nil
	}

	only := ctx.ReviewConfig.Branches.Only
	ignore := ctx.ReviewConfig.Branches.Ignore

	if only == "" && ignore == "" {
		return nil
	}

	branch := ctx.AppState.Branch

	if only != "" {
		matched, err := matchBranch(only, branch)
		if err != nil {
			return fmt.Errorf("branches.only pattern %q: %w", only, err)
		}
		if !matched {
			ctx.Logger.Log("branch %q does not match branches.only pattern %q, skipping deploy", branch, only)
			return fmt.Errorf("branch %q does not match allowed pattern %q", branch, only)
		}
		ctx.Logger.Log("branch %q matches branches.only pattern %q", branch, only)
	}

	if ignore != "" {
		matched, err := matchBranch(ignore, branch)
		if err != nil {
			return fmt.Errorf("branches.ignore pattern %q: %w", ignore, err)
		}
		if matched {
			ctx.Logger.Log("branch %q matches branches.ignore pattern %q, skipping deploy", branch, ignore)
			return fmt.Errorf("branch %q matches ignored pattern %q", branch, ignore)
		}
		ctx.Logger.Log("branch %q does not match branches.ignore pattern %q, proceeding", branch, ignore)
	}

	return nil
}

// matchBranch checks if a branch matches a pattern.
// Supports glob-style patterns (feature/*, dependabot/*) by converting to regex,
// or plain regex patterns.
func matchBranch(pattern, branch string) (bool, error) {
	// Convert glob-style wildcards to regex
	// feature/* → ^feature/.*$
	regex := "^" + globToRegex(pattern) + "$"
	return regexp.MatchString(regex, branch)
}

// globToRegex converts a simple glob pattern to a regex string.
// * matches any characters, ? matches a single character.
func globToRegex(glob string) string {
	var result []byte
	for i := 0; i < len(glob); i++ {
		switch glob[i] {
		case '*':
			result = append(result, '.', '*')
		case '?':
			result = append(result, '.')
		case '.', '(', ')', '+', '|', '^', '$', '[', ']', '{', '}', '\\':
			result = append(result, '\\', glob[i])
		default:
			result = append(result, glob[i])
		}
	}
	return string(result)
}
