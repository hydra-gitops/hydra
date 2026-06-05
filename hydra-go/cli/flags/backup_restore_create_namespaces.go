package flags

type WithBackupRestoreCreateNamespacesFlag interface {
	WithBackupRestoreCreateNamespacesFlag() *BackupRestoreCreateNamespacesFlag
}

type BackupRestoreCreateNamespacesFlag struct {
	CreateNamespaces bool
}

var _ Flags = (*BackupRestoreCreateNamespacesFlag)(nil)
var _ WithBackupRestoreCreateNamespacesFlag = (*BackupRestoreCreateNamespacesFlag)(nil)

func (f *BackupRestoreCreateNamespacesFlag) Flags() Flags {
	return f
}

func (f *BackupRestoreCreateNamespacesFlag) WithBackupRestoreCreateNamespacesFlag() *BackupRestoreCreateNamespacesFlag {
	return f
}
