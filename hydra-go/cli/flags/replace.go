package flags

type WithReplaceFlag interface {
	WithReplaceFlag() *ReplaceFlag
}

type ReplaceFlag struct {
	Replace bool
}

var _ Flags = (*ReplaceFlag)(nil)
var _ WithReplaceFlag = (*ReplaceFlag)(nil)

func (f *ReplaceFlag) Flags() Flags {
	return f
}

func (f *ReplaceFlag) WithReplaceFlag() *ReplaceFlag {
	return f
}
