package commands

import (
	"fmt"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yaml"
	"hydra-gitops.org/hydra/hydra-go/core/yq"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

// TemplatePatchPipeline applies global.hydra.templatePatches rules (CEL predicate + yq) to rendered manifests.
type TemplatePatchPipeline struct {
	rules            []compiledTemplatePatchRule
	ownerByNamespace map[types.Namespace]types.AppId // optional; enables DeclaringApp matching for synthetic kubernetes-defaults entities
}

type compiledTemplatePatchRule struct {
	name         string
	declaringApp types.AppId
	pred         cel.Predicate
	yqExprs      []string
}

// NewTemplatePatchPipeline compiles template patch rules. Returns an empty pipeline if entries is nil or empty.
func NewTemplatePatchPipeline(entries []types.TemplatePatchRuleEntry) (*TemplatePatchPipeline, error) {
	return NewTemplatePatchPipelineWithNamespaceOwners(entries, nil)
}

// NewTemplatePatchPipelineWithNamespaceOwners is like [NewTemplatePatchPipeline] but optionally supplies
// namespace owner ids (same semantics as [BuildNamespaceOwnerMap]) so templatePatches with DeclaringApp
// can apply to synthetic kubernetes-defaults entities and attribute AppIds after a successful patch.
func NewTemplatePatchPipelineWithNamespaceOwners(entries []types.TemplatePatchRuleEntry, ownerByNamespace map[types.Namespace]types.AppId) (*TemplatePatchPipeline, error) {
	if len(entries) == 0 {
		return &TemplatePatchPipeline{rules: nil, ownerByNamespace: ownerByNamespace}, nil
	}
	env, err := cel.NewEnv()
	if err != nil {
		return nil, err
	}
	var rules []compiledTemplatePatchRule
	for _, ent := range entries {
		p, err := env.CompilePredicateAt(fmt.Sprintf(`global.hydra.templatePatches rule %q`, ent.Name), types.CelPredicate(ent.Rule.Predicate))
		if err != nil {
			return nil, err
		}
		var exprs []string
		for _, patch := range ent.Rule.Patches {
			if patch.Yq == "" {
				return nil, log.CreateError(errors.ErrHydraConfigError,
					"templatePatches rule {name} has an empty yq patch",
					log.String("name", ent.Name))
			}
			exprs = append(exprs, patch.Yq)
		}
		if len(exprs) == 0 {
			return nil, log.CreateError(errors.ErrHydraConfigError,
				"templatePatches rule {name} must have at least one yq patch",
				log.String("name", ent.Name))
		}
		rules = append(rules, compiledTemplatePatchRule{
			name:         ent.Name,
			declaringApp: ent.DeclaringApp,
			pred:         p,
			yqExprs:      exprs,
		})
	}
	return &TemplatePatchPipeline{rules: rules, ownerByNamespace: ownerByNamespace}, nil
}

// BuildTemplatePatchOwnerByNamespace resolves namespace -> owning app id for template patch attribution
// on synthetic kubernetes-defaults resources. mergedSelectedAndCatalog should match the render merge
// used for collecting template patch rules (partition + Hydra ConfigMap catalog).
func BuildTemplatePatchOwnerByNamespace(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	partitionRender entity.Entities,
	hydraConfigCatalogRender entity.Entities,
	key types.EntityKeyUnstructured,
) (map[types.Namespace]types.AppId, error) {
	merged, err := hydra.MergeRenderedForHydraPartition(partitionRender, hydraConfigCatalogRender)
	if err != nil {
		return nil, err
	}
	allAppIds, err := cluster.AppIds(networkMode)
	if err != nil {
		return nil, err
	}
	declaredOwners, err := hydra.HydraAppNamespaceOwners(cluster, allAppIds, networkMode)
	if err != nil {
		return nil, err
	}
	return BuildNamespaceOwnerMap(merged, key, declaredOwners, merged)
}

type resourceIdentity struct {
	apiVersion string
	kind       string
	name       string
	namespace  string
}

func identityOfUnstructured(u *unstructured.Unstructured) resourceIdentity {
	return resourceIdentity{
		apiVersion: u.GetAPIVersion(),
		kind:       u.GetKind(),
		name:       u.GetName(),
		namespace:  u.GetNamespace(),
	}
}

func (a resourceIdentity) equals(b resourceIdentity) bool {
	return a.apiVersion == b.apiVersion && a.kind == b.kind && a.name == b.name && a.namespace == b.namespace
}

// ApplyTemplatePatchesToEntities runs the pipeline on each entity that carries unstructured data at key.
func ApplyTemplatePatchesToEntities(
	p *TemplatePatchPipeline,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
) (entity.Entities, error) {
	return applyTemplatePatchesToEntities(p, entities, key, false)
}

// ApplyTemplatePatchesToEntitiesBeforeScope runs templatePatches before Kubernetes scope validation.
// At this point cluster-scoped resources may still carry an invalid metadata.namespace from Helm output,
// so namespace identity changes are allowed and the following scope pass validates the result.
func ApplyTemplatePatchesToEntitiesBeforeScope(
	p *TemplatePatchPipeline,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
) (entity.Entities, error) {
	return applyTemplatePatchesToEntities(p, entities, key, true)
}

func applyTemplatePatchesToEntities(
	p *TemplatePatchPipeline,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	allowNamespaceIdentityChange bool,
) (entity.Entities, error) {
	if p == nil || len(p.rules) == 0 {
		return entities, nil
	}
	items := make([]entity.Entity, 0, len(entities.Items))
	for _, e := range entities.Items {
		u, ok := e.Unstructured(key)
		if !ok {
			items = append(items, e)
			continue
		}
		patched, err := applyTemplatePatchesToOne(p, e, key, u, allowNamespaceIdentityChange)
		if err != nil {
			return entity.Entities{}, err
		}
		items = append(items, patched)
	}
	return entity.NewEntities(items)
}

func templatePatchEntityNamespace(u *unstructured.Unstructured) (types.Namespace, bool) {
	if u.GetAPIVersion() == "v1" && u.GetKind() == "Namespace" {
		name := u.GetName()
		return types.Namespace(name), name != ""
	}
	ns := u.GetNamespace()
	return types.Namespace(ns), ns != ""
}

func applyTemplatePatchesToOne(
	p *TemplatePatchPipeline,
	e entity.Entity,
	key types.EntityKeyUnstructured,
	u unstructured.Unstructured,
	allowNamespaceIdentityChange bool,
) (entity.Entity, error) {
	primary, primaryOk := primaryDeclaringAppForTemplatePatch(e)

	syntheticNoPrimary := false
	if tp, err := e.TemplatePath(); err == nil {
		syntheticNoPrimary = strings.Contains(string(tp), "kubernetes-defaults-") && !primaryOk
	}
	nsForOwner, nsOk := templatePatchEntityNamespace(&u)

	protected := hydra.IsHydraConfigDataConfigMap(&u)
	var beforeYaml types.YamlString
	var err error
	if protected {
		beforeYaml, err = yaml.ToYaml(u.Object)
		if err != nil {
			return entity.Entity{}, err
		}
	}

	idBefore := identityOfUnstructured(&u)
	ys, err := yaml.ToYaml(u.Object)
	if err != nil {
		return entity.Entity{}, err
	}
	initialYs := ys

	declaringOwners := sets.New[types.AppId]()
	globalRuleChangedYAML := false

	for _, rule := range p.rules {
		if rule.declaringApp != "" {
			if primaryOk {
				if rule.declaringApp != primary {
					continue
				}
			} else if syntheticNoPrimary && p.ownerByNamespace != nil && nsOk {
				ow, ok := p.ownerByNamespace[nsForOwner]
				if !ok || ow != rule.declaringApp {
					continue
				}
			} else {
				continue
			}
		}
		match, err := rule.pred.EvalBool(e, types.MissingKeysReject)
		if err != nil {
			return entity.Entity{}, err
		}
		if !match {
			continue
		}
		beforeRule := ys
		for _, expr := range rule.yqExprs {
			ys, err = yq.Yq(ys, expr)
			if err != nil {
				return entity.Entity{}, log.CreateError(errors.ErrYqFailed,
					"templatePatches rule {name} yq failed: {err}",
					log.String("name", rule.name), log.Err(err))
			}
		}
		if string(beforeRule) != string(ys) {
			if rule.declaringApp != "" {
				declaringOwners.Insert(rule.declaringApp)
			} else {
				globalRuleChangedYAML = true
			}
		}
	}

	obj, err := yaml.FromYaml[map[string]any](ys)
	if err != nil {
		return entity.Entity{}, err
	}
	obj = yaml.NormalizeUnstructuredObjectForDeepCopy(obj)
	uOut := unstructured.Unstructured{Object: obj}
	idAfter := identityOfUnstructured(&uOut)
	sameIdentityExceptNamespace := idBefore.apiVersion == idAfter.apiVersion &&
		idBefore.kind == idAfter.kind &&
		idBefore.name == idAfter.name
	if !idBefore.equals(idAfter) && !(allowNamespaceIdentityChange && sameIdentityExceptNamespace) {
		idStr, idErr := e.Id()
		if idErr != nil {
			idStr = ""
		}
		return entity.Entity{}, log.CreateError(errors.ErrHydraConfigError,
			"templatePatches changed resource identity (apiVersion, kind, metadata.name, or metadata.namespace) for {id}",
			log.String("id", string(idStr)))
	}

	if protected {
		afterYaml, err := yaml.ToYaml(uOut.Object)
		if err != nil {
			return entity.Entity{}, err
		}
		if string(beforeYaml) != string(afterYaml) {
			idStr, idErr := e.Id()
			if idErr != nil {
				idStr = ""
			}
			return entity.Entity{}, log.CreateError(errors.ErrHydraConfigError,
				"templatePatches must not mutate Hydra configuration ConfigMap {id}",
				log.String("id", string(idStr)))
		}
	}

	changed := string(initialYs) != string(ys)
	var assignedApp types.AppId
	if syntheticNoPrimary && changed {
		switch {
		case declaringOwners.Len() > 1:
			idStr, idErr := e.Id()
			if idErr != nil {
				idStr = ""
			}
			return entity.Entity{}, log.CreateError(errors.ErrHydraConfigError,
				"templatePatches: conflicting declaringApp rules patched the same synthetic kubernetes-defaults resource {id}",
				log.String("id", string(idStr)))
		case declaringOwners.Len() == 1:
			assignedApp = declaringOwners.UnsortedList()[0]
		case globalRuleChangedYAML:
			if p.ownerByNamespace == nil || !nsOk {
				idStr, idErr := e.Id()
				if idErr != nil {
					idStr = ""
				}
				return entity.Entity{}, log.CreateError(errors.ErrHydraConfigError,
					"templatePatches: global rules patched synthetic kubernetes-defaults resource {id} but namespace owner map is unavailable or namespace is missing",
					log.String("id", string(idStr)))
			}
			own, ok := p.ownerByNamespace[nsForOwner]
			if !ok || own == "" {
				idStr, idErr := e.Id()
				if idErr != nil {
					idStr = ""
				}
				return entity.Entity{}, log.CreateError(errors.ErrHydraConfigError,
					"templatePatches: global rules patched synthetic kubernetes-defaults resource {id} but namespace {ns} has no resolved owner app",
					log.String("id", string(idStr)), log.String("ns", string(nsForOwner)))
			}
			assignedApp = own
		default:
			idStr, idErr := e.Id()
			if idErr != nil {
				idStr = ""
			}
			return entity.Entity{}, log.CreateError(errors.ErrHydraConfigError,
				"templatePatches: synthetic kubernetes-defaults resource {id} changed without attributable rule",
				log.String("id", string(idStr)))
		}
	}

	return e.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
		b = b.WithUnstructured(key, uOut)
		if assignedApp != "" {
			b = b.WithAppIds([]types.AppId{assignedApp})
		}
		return b
	})
}

func primaryDeclaringAppForTemplatePatch(e entity.Entity) (types.AppId, bool) {
	ids, err := e.AppIds()
	if err != nil || len(ids) == 0 {
		return "", false
	}
	return ids[0], true
}

// ApplyTemplatePatchesUsingPartitionRender collects templatePatches from partitionRender (unpatched)
// and applies them to entities. selectedAppIds must match the app set used to build partitionRender
// for per-app Helm + ConfigMap merge semantics. hydraConfigCatalogRender lists extra template entities
// (typically all cluster apps) so Hydra ConfigMaps from apps outside the partition are still merged;
// pass entity.Entities{} when partitionRender already includes every cluster app.
func ApplyTemplatePatchesUsingPartitionRender(
	cluster *hydra.Cluster,
	selectedAppIds sets.Set[types.AppId],
	networkMode types.HelmNetworkMode,
	partitionRender entity.Entities,
	hydraConfigCatalogRender entity.Entities,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
) (entity.Entities, error) {
	entries, err := hydra.HydraTemplatePatchRuleEntries(cluster, selectedAppIds, networkMode, partitionRender, hydraConfigCatalogRender)
	if err != nil {
		return entity.Entities{}, err
	}
	if len(entries) == 0 {
		return entities, nil
	}
	ownerByNs, err := BuildTemplatePatchOwnerByNamespace(cluster, networkMode, partitionRender, hydraConfigCatalogRender, key)
	if err != nil {
		return entity.Entities{}, err
	}
	pipe, err := NewTemplatePatchPipelineWithNamespaceOwners(entries, ownerByNs)
	if err != nil {
		return entity.Entities{}, err
	}
	return ApplyTemplatePatchesToEntities(pipe, entities, key)
}
