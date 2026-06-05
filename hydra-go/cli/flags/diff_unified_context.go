package flags

// WithDiffUnifiedContextFlag is implemented by command flag structs that support
// grep-style unified-diff context (-A / -B / -C).
type WithDiffUnifiedContextFlag interface {
	WithDiffUnifiedContextFlag() *DiffUnifiedContextFlag
}

// DiffUnifiedContextFlag holds optional context line counts for unified diff output.
// Each field uses -1 for "flag not set". Values >= 0 are explicit user settings.
type DiffUnifiedContextFlag struct {
	Before int
	After  int
	Both   int
}

var _ Flags = (*DiffUnifiedContextFlag)(nil)
var _ WithDiffUnifiedContextFlag = (*DiffUnifiedContextFlag)(nil)

func (f *DiffUnifiedContextFlag) Flags() Flags {
	return f
}

func (f *DiffUnifiedContextFlag) WithDiffUnifiedContextFlag() *DiffUnifiedContextFlag {
	return f
}

// UnifiedDiffContextLines returns the Context value for go-difflib.UnifiedDiff.
// Unset fields (-1) are ignored; if none are set, the default is 3. If any field
// is set (>= 0), the result is the maximum of those values, matching grep-style
// combination of -A, -B, and -C. The underlying diff library only supports
// symmetric context, so asymmetric -A/-B are approximated by this maximum.
func UnifiedDiffContextLines(before, after, both int) int {
	max := -1
	for _, v := range []int{before, after, both} {
		if v >= 0 && v > max {
			max = v
		}
	}
	if max < 0 {
		return 3
	}
	return max
}
