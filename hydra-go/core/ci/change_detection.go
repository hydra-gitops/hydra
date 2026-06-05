package ci

// ChangedChart represents a chart directory that has been detected as
// changed since its last build tag.
type ChangedChart struct {
	Group string // e.g. "demo"
	App   string // e.g. "service-ui"
	Env   string // e.g. "dev"
	Path  string // e.g. "apps/demo/service-ui/dev"
}

// PromoteBranch returns the branch name for a promote operation
// following the convention: hydra/promote/to-<target>/<group>/<app>
func PromoteBranch(targetEnv, group, app string) string {
	return "hydra/promote/to-" + targetEnv + "/" + group + "/" + app
}

// BuildTagPrefix is the prefix used for build tags that trigger package.
const BuildTagPrefix = "build-"

// AppTag creates a documentation tag in the format <group>-<app>-<version>.
func AppTag(group, app, version string) string {
	return group + "-" + app + "-" + version
}

// RootAppTag creates a documentation tag for a root app: <group>-root-<version>.
func RootAppTag(group, version string) string {
	return group + "-root-" + version
}
