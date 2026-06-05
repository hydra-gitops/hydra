package types

type Label string
type LabelValue string

const (
	LabelAppName      Label = "app.kubernetes.io/name"
	LabelAppInstance  Label = "app.kubernetes.io/instance"
	LabelAppComponent Label = "app.kubernetes.io/component"
	LabelAppPartOf    Label = "app.kubernetes.io/part-of"
	LabelAppManagedBy Label = "app.kubernetes.io/managed-by"
	LabelAppVersion   Label = "app.kubernetes.io/version"

	LabelRioOwnerNamespace Label = "objectset.rio.cattle.io/owner-namespace"
	LabelRioOwnerGvk       Label = "objectset.rio.cattle.io/owner-gvk"
	LabelRioOwnerName      Label = "objectset.rio.cattle.io/owner-name"
)
