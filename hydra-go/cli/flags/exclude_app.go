package flags

import "hydra-gitops.org/hydra/hydra-go/core/types"

type WithExcludeAppFlag interface {
	WithExcludeAppFlag() *ExcludeAppFlag
}

type ExcludeAppFlag struct {
	ExcludeAppPatterns []types.AppIdPattern
}

var _ Flags = (*ExcludeAppFlag)(nil)
var _ WithExcludeAppFlag = (*ExcludeAppFlag)(nil)

func (f *ExcludeAppFlag) Flags() Flags {
	return f
}

func (f *ExcludeAppFlag) WithExcludeAppFlag() *ExcludeAppFlag {
	return f
}
