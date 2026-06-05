package flags

import "hydra-gitops.org/hydra/hydra-go/core/types"

type WithAppIdFlag interface {
	WithAppIdFlag() *AppIdFlag
}

type AppIdFlag struct {
	AppId types.AppId
}

var _ Flags = (*AppIdFlag)(nil)
var _ WithAppIdFlag = (*AppIdFlag)(nil)

func (f *AppIdFlag) Flags() Flags {
	return f
}

func (f *AppIdFlag) WithAppIdFlag() *AppIdFlag {
	return f
}
