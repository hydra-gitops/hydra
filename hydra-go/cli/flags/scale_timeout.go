package flags

import "time"

type WithScaleTimeoutFlag interface {
	WithScaleTimeoutFlag() *ScaleTimeoutFlag
}

type ScaleTimeoutFlag struct {
	ScaleTimeout time.Duration
}

var _ Flags = (*ScaleTimeoutFlag)(nil)
var _ WithScaleTimeoutFlag = (*ScaleTimeoutFlag)(nil)

func (f *ScaleTimeoutFlag) Flags() Flags {
	return f
}

func (f *ScaleTimeoutFlag) WithScaleTimeoutFlag() *ScaleTimeoutFlag {
	return f
}
