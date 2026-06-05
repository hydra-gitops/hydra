package types

import (
	"regexp"
	"strings"
)

// MatchAppIdGlob tests whether name matches a glob pattern where
// '*' matches zero or more non-dot characters and '**' matches zero or
// more characters including dots. Same semantics as hydra gitops CLI app patterns.
func MatchAppIdGlob(pattern, name string) bool {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); {
		if i+1 < len(pattern) && pattern[i] == '*' && pattern[i+1] == '*' {
			b.WriteString(".*")
			i += 2
		} else if pattern[i] == '*' {
			b.WriteString("[^.]*")
			i++
		} else {
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
			i++
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(name)
}

// IsGlobPattern returns true when the pattern contains at least one '*'.
func IsGlobPattern(pattern string) bool {
	return strings.Contains(pattern, "*")
}
