package git

import (
	"fmt"
	"sort"

	"github.com/go-git/go-git/v5/plumbing"
)

// TagsPointingTo returns all tag names whose peeled commit matches the commit
// resolved from ref (for example "HEAD" or a branch name). Tag names are
// sorted lexicographically.
func (r *Repo) TagsPointingTo(ref string) ([]string, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	target, err := r.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, fmt.Errorf("resolve ref %q: %w", ref, err)
	}
	iter, err := r.repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	var out []string
	err = iter.ForEach(func(tagRef *plumbing.Reference) error {
		name := tagRef.Name()
		if !name.IsTag() {
			return nil
		}
		h, err2 := r.repo.ResolveRevision(plumbing.Revision(name.String()))
		if err2 != nil {
			return nil
		}
		if *h != *target {
			return nil
		}
		out = append(out, name.Short())
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// CommitHash resolves ref (for example "HEAD", "main", or a tag name) to the
// peeled commit hash string.
func (r *Repo) CommitHash(ref string) (string, error) {
	if r.Err != nil {
		return "", r.Err
	}
	hash, err := r.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return "", fmt.Errorf("resolve ref %q: %w", ref, err)
	}
	return hash.String(), nil
}
