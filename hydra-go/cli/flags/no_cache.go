package flags

// WithNoCacheFlag is implemented by flag structs that support --no-cache.
type WithNoCacheFlag interface {
	WithNoCacheFlag() *NoCacheFlag
}

// NoCacheFlag disables persistent Helm template caching and related in-process caches.
type NoCacheFlag struct {
	NoCache bool
}

var _ Flags = (*NoCacheFlag)(nil)
var _ WithNoCacheFlag = (*NoCacheFlag)(nil)

func (f *NoCacheFlag) Flags() Flags {
	return f
}

func (f *NoCacheFlag) WithNoCacheFlag() *NoCacheFlag {
	return f
}
