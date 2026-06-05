package flags

type WithForceBackupRestoreFlag interface {
	WithForceBackupRestoreFlag() *ForceBackupRestoreFlag
}

type ForceBackupRestoreFlag struct {
	ForceBackupRestore bool
}

var _ Flags = (*ForceBackupRestoreFlag)(nil)
var _ WithForceBackupRestoreFlag = (*ForceBackupRestoreFlag)(nil)

func (f *ForceBackupRestoreFlag) Flags() Flags {
	return f
}

func (f *ForceBackupRestoreFlag) WithForceBackupRestoreFlag() *ForceBackupRestoreFlag {
	return f
}
