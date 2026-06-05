package flags

type WithUniqFlag interface {
	WithUniqFlag() *UniqFlag
}

type UniqFlag struct {
	Uniq bool
}

var _ WithUniqFlag = (*UniqFlag)(nil)

func (f *UniqFlag) WithUniqFlag() *UniqFlag {
	return f
}
