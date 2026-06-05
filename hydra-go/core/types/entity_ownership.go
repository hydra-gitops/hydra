package types

type EntityOwnership string

const (
	EntityOwnershipAppOwned  EntityOwnership = "AppOwned"
	EntityOwnershipBuiltIn   EntityOwnership = "BuiltIn"
	EntityOwnershipUntracked EntityOwnership = "Untracked"
)
