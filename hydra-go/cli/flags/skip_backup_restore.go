package flags

type WithSkipBackupRestoreFlag interface {
	WithSkipBackupRestoreFlag() *SkipBackupRestoreFlag
}

type SkipBackupRestoreFlag struct {
	SkipBackupRestore bool
}

var _ Flags = (*SkipBackupRestoreFlag)(nil)
var _ WithSkipBackupRestoreFlag = (*SkipBackupRestoreFlag)(nil)

func (f *SkipBackupRestoreFlag) Flags() Flags {
	return f
}

func (f *SkipBackupRestoreFlag) WithSkipBackupRestoreFlag() *SkipBackupRestoreFlag {
	return f
}
