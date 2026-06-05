package flags

type WithSkipBackupFlag interface {
	WithSkipBackupFlag() *SkipBackupFlag
}

type SkipBackupFlag struct {
	SkipBackup bool
}

var _ Flags = (*SkipBackupFlag)(nil)
var _ WithSkipBackupFlag = (*SkipBackupFlag)(nil)

func (f *SkipBackupFlag) Flags() Flags {
	return f
}

func (f *SkipBackupFlag) WithSkipBackupFlag() *SkipBackupFlag {
	return f
}
