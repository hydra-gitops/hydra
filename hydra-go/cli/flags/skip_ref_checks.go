package flags

type WithSkipRefChecksFlag interface {
	WithSkipRefChecksFlag() *SkipRefChecksFlag
}

type SkipRefChecksFlag struct {
	SkipRefChecks bool
}

var _ Flags = (*SkipRefChecksFlag)(nil)
var _ WithSkipRefChecksFlag = (*SkipRefChecksFlag)(nil)

func (f *SkipRefChecksFlag) Flags() Flags {
	return f
}

func (f *SkipRefChecksFlag) WithSkipRefChecksFlag() *SkipRefChecksFlag {
	return f
}
