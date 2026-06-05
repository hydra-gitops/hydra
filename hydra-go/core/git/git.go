package git

// RemoteURL returns the URL of the "origin" remote for the git repository
// containing dir. Returns ("", err) if dir is not inside a git repository or
// the remote is not configured.
func RemoteURL(dir string) (string, error) {
	r := Open(dir)
	if r.Err != nil {
		return "", r.Err
	}
	return r.RemoteURL()
}

// RepoRoot returns the absolute path of the top-level directory of the git
// repository containing dir.
func RepoRoot(dir string) (string, error) {
	r := Open(dir)
	if r.Err != nil {
		return "", r.Err
	}
	return r.Path(), nil
}

// Branch returns the current branch name for the git repository containing
// dir. Returns "HEAD" when in detached-HEAD state.
func Branch(dir string) (string, error) {
	r := Open(dir)
	if r.Err != nil {
		return "", r.Err
	}
	return r.CurrentBranch()
}
