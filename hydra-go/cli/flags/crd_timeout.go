package flags

import "time"

type WithCrdTimeoutFlag interface {
	WithCrdTimeoutFlag() *CrdTimeoutFlag
}

type CrdTimeoutFlag struct {
	CrdTimeout time.Duration
}

var _ Flags = (*CrdTimeoutFlag)(nil)
var _ WithCrdTimeoutFlag = (*CrdTimeoutFlag)(nil)

func (f *CrdTimeoutFlag) Flags() Flags {
	return f
}

func (f *CrdTimeoutFlag) WithCrdTimeoutFlag() *CrdTimeoutFlag {
	return f
}
