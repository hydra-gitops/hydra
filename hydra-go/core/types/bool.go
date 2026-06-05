package types

type Direction bool

const (
	DirectionAscending  Direction = false
	DirectionDescending Direction = true
)

type KeepServerFields bool

const (
	KeepServerFieldsYes KeepServerFields = true
	KeepServerFieldsNo  KeepServerFields = false
)

type DryRun bool

const (
	DryRunYes DryRun = true
	DryRunNo  DryRun = false
)

func (d DryRun) Mode() string {
	if d {
		return "Dry Run Mode"
	}
	return "Normal Mode"
}

type Namespaced bool

const (
	NamespacedYes Namespaced = false
	NamespacedNo  Namespaced = true
)

type Selected bool

const (
	SelectedYes Selected = true
	SelectedNo  Selected = false
)

type MissingKeys int

const (
	MissingKeysError  MissingKeys = 0
	MissingKeysAccept MissingKeys = 1
	MissingKeysReject MissingKeys = 2
)

type KubernetesConnectionAllowed bool

const (
	KubernetesConnectionAllowedYes KubernetesConnectionAllowed = true
	KubernetesConnectionAllowedNo  KubernetesConnectionAllowed = false
)

type SkipRootApps bool

const (
	SkipRootAppsYes SkipRootApps = true
	SkipRootAppsNo  SkipRootApps = false
)
