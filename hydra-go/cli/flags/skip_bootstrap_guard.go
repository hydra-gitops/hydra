package flags

type WithSkipBootstrapGuardFlag interface {
	WithSkipBootstrapGuardFlag() *SkipBootstrapGuardFlag
}

type SkipBootstrapGuardFlag struct {
	SkipBootstrapGuard bool
}

var _ Flags = (*SkipBootstrapGuardFlag)(nil)
var _ WithSkipBootstrapGuardFlag = (*SkipBootstrapGuardFlag)(nil)

func (f *SkipBootstrapGuardFlag) Flags() Flags {
	return f
}

func (f *SkipBootstrapGuardFlag) WithSkipBootstrapGuardFlag() *SkipBootstrapGuardFlag {
	return f
}
