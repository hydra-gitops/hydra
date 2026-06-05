package log

import "strings"

// LogId represents a hierarchical logging identifier.
// It supports parent-child relationships for structured logging.
type LogId struct {
	name   string
	parent *LogId
}

// newLogId creates a new root LogId with the given name.
func newLogId(name string) LogId {
	return LogId{name: name}
}

// Child creates a child LogId with this LogId as parent.
func (id LogId) Child(name string) LogId {
	return LogId{name: name, parent: &id}
}

// Clone creates a deep copy of the LogId, recursively cloning all parents.
func (id LogId) Clone() LogId {
	if id.parent == nil {
		return LogId{name: id.name}
	}
	parentClone := id.parent.Clone()
	return LogId{name: id.name, parent: &parentClone}
}

// String returns the full path of the LogId (e.g., "hydra.core.hydra.Context").
func (id LogId) String() string {
	if id.parent == nil {
		return id.name
	}
	return id.parent.String() + "." + id.name
}

// Name returns just the name of this LogId without parents.
func (id LogId) Name() string {
	return id.name
}

// Path returns the LogId hierarchy as a slice from root to this LogId.
func (id LogId) Path() []string {
	if id.parent == nil {
		return []string{id.name}
	}
	return append(id.parent.Path(), id.name)
}

// Depth returns the depth of this LogId in the hierarchy (root = 0).
func (id LogId) Depth() int {
	if id.parent == nil {
		return 0
	}
	return id.parent.Depth() + 1
}

// ShortString returns a shortened version for display (last 2 components).
func (id LogId) ShortString() string {
	path := id.Path()
	if len(path) <= 2 {
		return strings.Join(path, ".")
	}
	return strings.Join(path[len(path)-2:], ".")
}

// Private root-level LogIds
var hydra = newLogId("hydra")
var base = hydra.Child("base")

// Hydra returns a clone of the root hydra LogId.
func Hydra() LogId {
	return hydra.Clone()
}

// Base returns a clone of the base module LogId.
func Base() LogId {
	return base.Clone()
}
