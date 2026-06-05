package commands

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yaml"
	"hydra-gitops.org/hydra/hydra-go/core/yq"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DiffIgnorePipeline applies global.hydra.diff.ignore rules (CEL predicate + yq patches) before YAML comparison.
type DiffIgnorePipeline struct {
	rules []compiledDiffIgnoreRule
}

type compiledDiffIgnoreRule struct {
	name                       string
	pred                       cel.Predicate
	yqExprs                    []string
	ignoreWhenMissingInCluster bool
}

// NewDiffIgnorePipeline compiles diff ignore rules. Returns an empty pipeline if entries is nil or empty.
func NewDiffIgnorePipeline(entries []types.DiffIgnoreRuleEntry) (*DiffIgnorePipeline, error) {
	if len(entries) == 0 {
		return &DiffIgnorePipeline{rules: nil}, nil
	}
	env, err := cel.NewEnv()
	if err != nil {
		return nil, err
	}
	var rules []compiledDiffIgnoreRule
	for _, ent := range entries {
		p, err := env.CompilePredicateAt(fmt.Sprintf(`global.hydra.diff.ignore rule %q`, ent.Name), types.CelPredicate(ent.Rule.Predicate))
		if err != nil {
			return nil, err
		}
		var exprs []string
		for _, patch := range ent.Rule.Patches {
			if patch.Yq == "" {
				return nil, log.CreateError(errors.ErrHydraConfigError,
					"diff.ignore rule {name} has an empty yq patch",
					log.String("name", ent.Name))
			}
			exprs = append(exprs, patch.Yq)
		}
		if len(exprs) == 0 && !ent.Rule.IgnoreWhenMissingInCluster {
			return nil, log.CreateError(errors.ErrHydraConfigError,
				"diff.ignore rule {name} must have at least one non-empty yq patch, or set ignoreWhenMissingInCluster: true",
				log.String("name", ent.Name))
		}
		rules = append(rules, compiledDiffIgnoreRule{
			name:                       ent.Name,
			pred:                       p,
			yqExprs:                    exprs,
			ignoreWhenMissingInCluster: ent.Rule.IgnoreWhenMissingInCluster,
		})
	}
	return &DiffIgnorePipeline{rules: rules}, nil
}

// ApplyToUnstructured runs all matching rules' yq expressions on a deep copy of u.Object, replacing u.Object in place.
func (p *DiffIgnorePipeline) ApplyToUnstructured(e entity.Entity, u *unstructured.Unstructured) error {
	if p == nil || len(p.rules) == 0 || u == nil {
		return nil
	}
	ys, err := yaml.ToYaml(u.Object)
	if err != nil {
		return err
	}
	for _, rule := range p.rules {
		match, err := rule.pred.EvalBool(e, types.MissingKeysReject)
		if err != nil {
			return err
		}
		if !match {
			continue
		}
		if len(rule.yqExprs) == 0 {
			continue
		}
		for _, expr := range rule.yqExprs {
			ys, err = yq.Yq(ys, expr)
			if err != nil {
				return log.CreateError(errors.ErrYqFailed,
					"diff.ignore rule {name} yq failed: {err}",
					log.String("name", rule.name), log.Err(err))
			}
		}
	}
	obj, err := yaml.FromYaml[map[string]any](ys)
	if err != nil {
		return err
	}
	u.Object = obj
	return nil
}

// IgnoreLeftOnlyWhenClusterMissing returns true when a matching diff.ignore rule has
// IgnoreWhenMissingInCluster and the CEL predicate matches. Used by hydra gitops diff
// to replace the unified diff for template-only resources with a short "diff ignored" line.
func (p *DiffIgnorePipeline) IgnoreLeftOnlyWhenClusterMissing(e entity.Entity) (bool, error) {
	if p == nil || len(p.rules) == 0 {
		return false, nil
	}
	for _, rule := range p.rules {
		if !rule.ignoreWhenMissingInCluster {
			continue
		}
		match, err := rule.pred.EvalBool(e, types.MissingKeysReject)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}
