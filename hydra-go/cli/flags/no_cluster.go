package flags

type WithNoClusterFlag interface {
	WithNoClusterFlag() *NoClusterFlag
}

type NoClusterFlag struct {
	NoCluster bool
}

var _ Flags = (*NoClusterFlag)(nil)
var _ WithNoClusterFlag = (*NoClusterFlag)(nil)

func (f *NoClusterFlag) Flags() Flags {
	return f
}

func (f *NoClusterFlag) WithNoClusterFlag() *NoClusterFlag {
	return f
}
