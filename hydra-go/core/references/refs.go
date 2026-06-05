package references

import (
	"cmp"
	"embed"
	stderrors "errors"
	"fmt"
	"io/fs"
	"reflect"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	goocel "github.com/google/cel-go/cel"
	celTypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/traits"
	"gopkg.in/yaml.v3"
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

//go:embed all:ref-parsers
var refParsersFS embed.FS

func ParseRefParsers(data []byte) ([]types.RefParser, error) {
	var config struct {
		RefParsers []struct {
			types.RefSelectorInput `yaml:",inline"`
			Pick                   []types.HydraRefPick       `yaml:"pick"`
			Tag                    []string                   `yaml:"tag"`
			Desc                   string                     `yaml:"desc"`
			Label                  string                     `yaml:"label"`
			Reverse                bool                       `yaml:"reverse,omitempty"`
			Attributes             []types.RefParserAttribute `yaml:"attributes"`
		} `yaml:"ref-parsers"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	result := make([]types.RefParser, 0, len(config.RefParsers))
	for i, rp := range config.RefParsers {
		selector, cel, err := rp.RefSelectorInput.Normalized()
		if err != nil {
			return nil, fmt.Errorf("ref-parsers[%d]: %w", i, err)
		}
		attributes, err := types.RefAttributesFromParserAttributes(rp.Attributes)
		if err != nil {
			return nil, err
		}
		pick := make([]types.RefPicker, 0, len(rp.Pick))
		for j, p := range rp.Pick {
			if strings.TrimSpace(p.Cel) == "" {
				return nil, fmt.Errorf("ref-parsers[%d].pick[%d]: cel is required", i, j)
			}
			pickAttrs, err := types.RefAttributesFromParserAttributes(p.Attributes)
			if err != nil {
				return nil, err
			}
			pick = append(pick, types.RefPicker{
				Cel:        types.CelExpression(p.Cel),
				Tag:        p.Tag,
				Label:      p.Label,
				Attributes: pickAttrs,
				Reverse:    p.Reverse,
			})
		}
		result = append(result, types.RefParser{
			Selector:   selector,
			Cel:        cel,
			Pick:       pick,
			Tags:       rp.Tag,
			Desc:       rp.Desc,
			Label:      rp.Label,
			Attributes: attributes,
			Reverse:    rp.Reverse,
		})
	}

	return result, nil
}

func DefaultRefParsers() []types.RefParser {
	entries := DefaultRefParserEntries()
	result := make([]types.RefParser, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry.Parser)
	}
	return result
}

type DefaultRefParserEntry struct {
	Parser     types.RefParser
	SourcePath string
}

func DefaultRefParserEntries() []DefaultRefParserEntry {
	var entries []DefaultRefParserEntry

	err := fs.WalkDir(refParsersFS, "ref-parsers", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		data, err := refParsersFS.ReadFile(path)
		if err != nil {
			return err
		}

		parsers, err := ParseRefParsers(data)
		if err != nil {
			return err
		}

		for _, parser := range parsers {
			entries = append(entries, DefaultRefParserEntry{
				Parser:     parser,
				SourcePath: path,
			})
		}
		return nil
	})

	if err != nil {
		panic(err)
	}

	return entries
}

// normalizeRefDefinitionEndpoints rewrites id endpoints to preferred API versions for group/kind
// so incoming/outgoing matching aligns with normalized entity IDs.
func normalizeRefDefinitionEndpoints(
	refDefs []types.RefDefinition,
	preferredVersions map[types.GroupKindKey]types.Version,
) []types.RefDefinition {
	if len(preferredVersions) == 0 || len(refDefs) == 0 {
		return refDefs
	}
	out := make([]types.RefDefinition, len(refDefs))
	copy(out, refDefs)
	for i := range out {
		if out[i].Endpoint.Type != types.RefEndpointTypeId {
			continue
		}
		group, version, kind, ns, name, err := types.Id(out[i].Endpoint.Value).Components()
		if err != nil {
			continue
		}
		gk := types.NewGroupKindKey(group, kind)
		preferred, ok := preferredVersions[gk]
		if !ok || preferred == version {
			continue
		}
		out[i].Endpoint.Value = string(types.NewId(group, preferred, kind, ns, name))
	}
	return out
}

// Refs finds all references in the given entities and returns them as a slice of types.Ref.
// preferredVersions maps group/kind to the cluster-preferred API version; when non-nil, id endpoints
// in ref definitions are rewritten to that version before incoming/outgoing matching. Pass nil when
// no normalization applies (for example tests or callers without cluster discovery).
func Refs(
	l log.Logger,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	envOpts []goocel.EnvOption,
	managedNamespaceSource entity.Entities,
	clusterInventoryOverlay entity.Entities,
	preferredVersions map[types.GroupKindKey]types.Version,
	extraParsers ...[]types.RefParser,
) ([]types.Ref, error) {
	return RefsWithProgress(l, entities, key, envOpts, managedNamespaceSource, clusterInventoryOverlay, preferredVersions, nil, extraParsers...)
}

// RefsProgress receives coarse progress from the ref matching phase after ref definitions have
// been extracted. Done is 1-based and total may change as later phases discover more work.
type RefsProgress func(done int, total int, detail string)

func RefsWithProgress(
	l log.Logger,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	envOpts []goocel.EnvOption,
	managedNamespaceSource entity.Entities,
	clusterInventoryOverlay entity.Entities,
	preferredVersions map[types.GroupKindKey]types.Version,
	progress RefsProgress,
	extraParsers ...[]types.RefParser,
) ([]types.Ref, error) {
	_, refs, err := resolveVirtualRefs(l, entities, key, envOpts, managedNamespaceSource, clusterInventoryOverlay, preferredVersions, progress, extraParsers...)
	return refs, err
}

// ResolveVirtualRefs returns the stabilized entity set and refs after recursively materializing
// virtual targets from generator/provisioned refs until a fixpoint is reached.
// Cluster inventory overlay for CEL clusterEntities() is only applied by [Refs] (for example cluster
// review source parsing), not by this fixpoint helper.
func ResolveVirtualRefs(
	l log.Logger,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	envOpts []goocel.EnvOption,
	managedNamespaceSource entity.Entities,
	preferredVersions map[types.GroupKindKey]types.Version,
	extraParsers ...[]types.RefParser,
) (entity.Entities, []types.Ref, error) {
	return resolveVirtualRefs(l, entities, key, envOpts, managedNamespaceSource, entity.Entities{}, preferredVersions, nil, extraParsers...)
}

func resolveVirtualRefs(
	l log.Logger,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	envOpts []goocel.EnvOption,
	managedNamespaceSource entity.Entities,
	clusterInventoryOverlay entity.Entities,
	preferredVersions map[types.GroupKindKey]types.Version,
	progress RefsProgress,
	extraParsers ...[]types.RefParser,
) (entity.Entities, []types.Ref, error) {
	current := entities
	baseIDs := sets.New[types.Id](entities.IdList...)
	pass := 1
	for {
		l.DebugLog(logIdReferences, "resolve virtual refs: pass {pass} start",
			log.Int("pass", pass),
			log.Int("entities", current.Len()))
		refs, err := refsSinglePass(l, current, key, envOpts, managedNamespaceSource, clusterInventoryOverlay, preferredVersions, progress, extraParsers...)
		if err != nil {
			return entity.Entities{}, nil, err
		}
		l.DebugLog(logIdReferences, "resolve virtual refs: pass {pass} materialize virtual targets",
			log.Int("pass", pass),
			log.Int("refs", len(refs)))
		expanded, changed, err := materializeVirtualTargets(l, current, baseIDs, key, refs, progress)
		if err != nil {
			return entity.Entities{}, nil, err
		}
		l.DebugLog(logIdReferences, "resolve virtual refs: pass {pass} materialize complete",
			log.Int("pass", pass),
			log.Bool("changed", changed),
			log.Int("entities", expanded.Len()))
		if !changed {
			return expanded, refs, nil
		}
		current = expanded
		pass++
	}
}

func refsSinglePass(
	l log.Logger,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	envOpts []goocel.EnvOption,
	managedNamespaceSource entity.Entities,
	clusterInventoryOverlay entity.Entities,
	preferredVersions map[types.GroupKindKey]types.Version,
	progress RefsProgress,
	extraParsers ...[]types.RefParser,
) ([]types.Ref, error) {
	refDefs, err := RefDefinitionsWithProgress(l, entities, key, envOpts, managedNamespaceSource, clusterInventoryOverlay, progress, extraParsers...)
	if err != nil {
		var pe *PickerExprError
		if stderrors.As(err, &pe) {
			l.Error(logIdReferences, "invalid ref picker expression result",
				log.String("expr", pe.Expr),
				log.String("expected", pe.Expected),
				log.String("gotType", pe.GotType),
			)
		}
		return nil, err
	}

	refDefs = normalizeRefDefinitionEndpoints(refDefs, preferredVersions)
	l.DebugLog(logIdReferences, "refs single pass: normalized {count} ref definitions",
		log.Int("count", len(refDefs)))
	refProgress := func(done int, total int, detail string) {
		if progress != nil {
			progress(done, total, detail)
		}
	}

	// Build incoming map: for each entity ID, the set of RefEndpoints pointing to it
	incoming := make(map[types.Id]sets.Set[types.RefEndpoint])
	// Build outgoing map: for each RefEndpoint, the RefDefinitions pointing from it (includes Owner, Label, and Reverse)
	type outgoingEntry struct {
		Owner      types.Id
		Label      string
		Tags       []string
		Desc       string
		Attributes []types.RefAttribute
		Reverse    bool
		RefType    types.RefType
	}
	outgoing := make(map[types.RefEndpoint][]outgoingEntry)

	for i, rd := range refDefs {
		if len(refDefs) > 0 && (i%256 == 0 || i == len(refDefs)-1) {
			refProgress(i+1, len(refDefs), fmt.Sprintf("index ref definitions - %d / %d", i+1, len(refDefs)))
		}
		l.DebugLog(logIdReferences, "refDefinition",
			log.String("Owner", string(rd.Owner)),
			log.String("Type", string(rd.Type)),
			log.String("Direction", string(rd.Direction)),
			log.String("EndpointType", string(rd.Endpoint.Type)),
			log.String("Endpoint", string(rd.Endpoint.Value)),
			log.String("Label", string(rd.Label)),
			log.Bool("Reverse", rd.Reverse),
		)
		switch rd.Direction {
		case types.RefDirectionIncoming:
			if _, ok := incoming[rd.Owner]; !ok {
				incoming[rd.Owner] = sets.New[types.RefEndpoint]()
			}
			incoming[rd.Owner].Insert(rd.Endpoint)
		case types.RefDirectionOutgoing:
			rt := rd.Type
			if rt == "" {
				rt = types.RefTypeDirect
			}
			outgoing[rd.Endpoint] = append(outgoing[rd.Endpoint], outgoingEntry{
				Owner:      rd.Owner,
				Label:      rd.Label,
				Tags:       rd.Tags,
				Desc:       rd.Desc,
				Attributes: rd.Attributes,
				Reverse:    rd.Reverse,
				RefType:    rt,
			})
		}
	}
	l.DebugLog(logIdReferences, "refs single pass: indexed ref definitions",
		log.Int("refDefinitions", len(refDefs)),
		log.Int("incomingOwners", len(incoming)),
		log.Int("outgoingEndpoints", len(outgoing)))

	// Build Refs by connecting incoming to outgoing
	// Collect all labels per edge and merge them
	type edgeKey struct {
		From    types.Id
		To      types.Id
		RefType types.RefType
	}
	type edgeData struct {
		EndpointType types.RefEndpointType
		Labels       sets.Set[string]
		Tags         sets.Set[string]
		Attributes   sets.Set[types.RefAttribute]
		Desc         string
		Reverse      bool
		RefType      types.RefType
	}
	edges := make(map[edgeKey]*edgeData)
	mergeEdge := func(key edgeKey, endpointType types.RefEndpointType, entry outgoingEntry) {
		if edges[key] == nil {
			rt := entry.RefType
			if rt == "" {
				rt = types.RefTypeDirect
			}
			edges[key] = &edgeData{
				EndpointType: endpointType,
				Labels:       sets.New[string](),
				Tags:         sets.New[string](),
				Attributes:   sets.New[types.RefAttribute](),
				Desc:         entry.Desc,
				Reverse:      entry.Reverse,
				RefType:      rt,
			}
		} else if entry.Desc != "" && edges[key].Desc != "" && entry.Desc != edges[key].Desc {
			l.Warn(logIdReferences, "Discarding conflicting desc during ref merge for edge {from} -> {to}: keeping '{kept}', discarding '{discarded}'",
				log.String("from", string(key.From)),
				log.String("to", string(key.To)),
				log.String("kept", edges[key].Desc),
				log.String("discarded", entry.Desc),
			)
		}
		if entry.Label != "" {
			edges[key].Labels.Insert(entry.Label)
		}
		edges[key].Tags.Insert(entry.Tags...)
		edges[key].Attributes.Insert(entry.Attributes...)
		if entry.Reverse {
			edges[key].Reverse = true
		}
	}

	incomingI := 0
	l.DebugLog(logIdReferences, "refs single pass: matching incoming endpoints",
		log.Int("incomingOwners", len(incoming)))
	for toId, incomingEndpoints := range incoming {
		incomingI++
		if len(incoming) > 0 && (incomingI%256 == 0 || incomingI == len(incoming)) {
			refProgress(incomingI, len(incoming), fmt.Sprintf("match incoming endpoints - %d / %d", incomingI, len(incoming)))
			l.DebugLog(logIdReferences, "refs single pass: matched incoming endpoint owners",
				log.Int("done", incomingI),
				log.Int("total", len(incoming)),
				log.Int("edges", len(edges)))
		}
		for endpoint := range incomingEndpoints {
			if entries, ok := outgoing[endpoint]; ok {
				for _, entry := range entries {
					rt := entry.RefType
					if rt == "" {
						rt = types.RefTypeDirect
					}
					mergeEdge(edgeKey{From: entry.Owner, To: toId, RefType: rt}, endpoint.Type, entry)
				}
			}
		}
	}
	l.DebugLog(logIdReferences, "refs single pass: incoming endpoint match complete",
		log.Int("edges", len(edges)))

	// Handle missing entities: outgoing refs of type "id" that have no corresponding incoming ref
	outgoingI := 0
	l.DebugLog(logIdReferences, "refs single pass: matching dangling id endpoints",
		log.Int("outgoingEndpoints", len(outgoing)))
	for endpoint, entries := range outgoing {
		outgoingI++
		if len(outgoing) > 0 && (outgoingI%256 == 0 || outgoingI == len(outgoing)) {
			refProgress(outgoingI, len(outgoing), fmt.Sprintf("match dangling id endpoints - %d / %d", outgoingI, len(outgoing)))
			l.DebugLog(logIdReferences, "refs single pass: matched dangling id endpoints",
				log.Int("done", outgoingI),
				log.Int("total", len(outgoing)),
				log.Int("edges", len(edges)))
		}
		if endpoint.Type != types.RefEndpointTypeId {
			continue
		}
		toId := types.Id(endpoint.Value)
		if _, exists := incoming[toId]; exists {
			continue
		}
		for _, entry := range entries {
			rt := entry.RefType
			if rt == "" {
				rt = types.RefTypeDirect
			}
			mergeEdge(edgeKey{From: entry.Owner, To: toId, RefType: rt}, endpoint.Type, entry)
		}
	}
	l.DebugLog(logIdReferences, "refs single pass: dangling id endpoint match complete",
		log.Int("edges", len(edges)))

	// Build result with merged labels and tags
	var result []types.Ref
	edgeI := 0
	l.DebugLog(logIdReferences, "refs single pass: building merged refs",
		log.Int("edges", len(edges)))
	for key, data := range edges {
		edgeI++
		if len(edges) > 0 && (edgeI%256 == 0 || edgeI == len(edges)) {
			refProgress(edgeI, len(edges), fmt.Sprintf("build merged refs - %d / %d", edgeI, len(edges)))
			l.DebugLog(logIdReferences, "refs single pass: built merged refs",
				log.Int("done", edgeI),
				log.Int("total", len(edges)))
		}
		labels := data.Labels.UnsortedList()
		slices.Sort(labels)
		tags := data.Tags.UnsortedList()
		slices.Sort(tags)
		rt := data.RefType
		if rt == "" {
			rt = types.RefTypeDirect
		}
		ref := types.Ref{
			RefType:      rt,
			EndpointType: data.EndpointType,
			From:         key.From,
			To:           key.To,
			Reverse:      data.Reverse,
		}
		if len(labels) > 0 {
			ref.Labels = labels
		}
		if len(tags) > 0 {
			ref.Tags = tags
		}
		attributes := edges[key].Attributes.UnsortedList()
		slices.SortFunc(attributes, func(a, b types.RefAttribute) int {
			if c := cmp.Compare(a.Type, b.Type); c != 0 {
				return c
			}
			return cmp.Compare(a.Value, b.Value)
		})
		if len(attributes) > 0 {
			ref.Attributes = attributes
		}
		if data.Desc != "" {
			ref.Desc = data.Desc
		}
		result = append(result, ref)
	}

	// Sort for deterministic output
	l.DebugLog(logIdReferences, "refs single pass: sorting refs",
		log.Int("refs", len(result)))
	refProgress(1, 1, fmt.Sprintf("sort refs - %d", len(result)))
	slices.SortFunc(result, func(a, b types.Ref) int {
		if c := cmp.Compare(a.From, b.From); c != 0 {
			return c
		}
		if c := cmp.Compare(a.To, b.To); c != 0 {
			return c
		}
		if c := cmp.Compare(a.RefType, b.RefType); c != 0 {
			return c
		}
		if c := cmp.Compare(a.EndpointType, b.EndpointType); c != 0 {
			return c
		}
		return cmp.Compare(strings.Join(a.Labels, ","), strings.Join(b.Labels, ","))
	})

	l.DebugLog(logIdReferences, "refs single pass: complete",
		log.Int("refs", len(result)))
	return result, nil
}

func RefDefinitions(entities entity.Entities, key types.EntityKeyUnstructured, envOpts []goocel.EnvOption, managedNamespaceSource entity.Entities, clusterInventoryOverlay entity.Entities, extraParsers ...[]types.RefParser) ([]types.RefDefinition, error) {
	return RefDefinitionsWithProgress(log.Default(), entities, key, envOpts, managedNamespaceSource, clusterInventoryOverlay, nil, extraParsers...)
}

func RefDefinitionsWithProgress(l log.Logger, entities entity.Entities, key types.EntityKeyUnstructured, envOpts []goocel.EnvOption, managedNamespaceSource entity.Entities, clusterInventoryOverlay entity.Entities, progress RefsProgress, extraParsers ...[]types.RefParser) ([]types.RefDefinition, error) {
	refProgress := func(done int, total int, detail string) {
		if progress != nil {
			progress(done, total, detail)
		}
	}

	l.DebugLog(logIdReferences, "ref definitions: prepare entities")
	entities, err := entities.CopyItems(key, types.KeyEntity)
	if err != nil {
		return nil, err
	}
	_, entities, err = entities.SelectByContainsEntityKey(key)
	if err != nil {
		return nil, err
	}
	l.DebugLog(logIdReferences, "ref definitions: selected {count} entities with key {key}",
		log.Int("count", len(entities.Items)),
		log.String("key", string(key)))

	// Build entity lookup map by Id
	entityMap := map[types.Id]entity.Entity{}
	for i, e := range entities.Items {
		if len(entities.Items) > 0 && (i%256 == 0 || i == len(entities.Items)-1) {
			refProgress(i+1, len(entities.Items), fmt.Sprintf("prepare entity lookup - %d / %d", i+1, len(entities.Items)))
		}
		id, err := e.Id()
		if err != nil {
			return nil, err
		}
		entityMap[id] = e
	}

	// Extract CRD-defined GVKs for the CRDs constant
	l.DebugLog(logIdReferences, "ref definitions: build CRD and service CEL support")
	scopeInfoMap, err := entities.ScopeInfoMapFromCrds(key)
	if err != nil {
		return nil, err
	}
	crdGVKs := make([]string, 0, len(scopeInfoMap))
	for gvk := range scopeInfoMap {
		crdGVKs = append(crdGVKs, string(gvk))
	}
	slices.Sort(crdGVKs)

	// Extract Services for label matching support
	servicesByNamespace := make(map[string][]cel.ServiceInfo)
	for i, e := range entities.Items {
		if len(entities.Items) > 0 && (i%256 == 0 || i == len(entities.Items)-1) {
			refProgress(i+1, len(entities.Items), fmt.Sprintf("scan service selectors - %d / %d", i+1, len(entities.Items)))
		}
		gvkStr, err := e.GVKString()
		if err != nil {
			continue
		}
		if gvkStr != "v1/Service" {
			continue
		}
		ns, err := e.Namespace()
		if err != nil {
			continue
		}
		name, err := e.Name()
		if err != nil {
			continue
		}
		u, ok := e.Unstructured(key)
		if !ok {
			continue
		}
		selector, _, _ := unstructured.NestedStringMap(u.Object, "spec", "selector")
		servicesByNamespace[string(ns)] = append(servicesByNamespace[string(ns)], cel.ServiceInfo{
			Id:       string(ns) + "/" + string(name),
			Selector: selector,
		})
	}

	preOpts := []goocel.EnvOption{cel.ListSupport("CRDs", crdGVKs), cel.ServiceSupport(servicesByNamespace)}
	preOpts = append(preOpts, envOpts...)
	l.DebugLog(logIdReferences, "ref definitions: compile pre-inventory CEL environment")
	refProgress(1, 4, "compile CEL environment - pre-inventory")
	tmpEnv, err := cel.NewEnv(preOpts...)
	if err != nil {
		return nil, err
	}
	l.DebugLog(logIdReferences, "ref definitions: build cluster inventory CEL support")
	refProgress(2, 4, "compile CEL environment - cluster inventory support")
	invOpt, err := cel.ClusterInventorySupport(tmpEnv, entities, managedNamespaceSource, clusterInventoryOverlay)
	if err != nil {
		return nil, err
	}
	finalOpts := append(preOpts, invOpt)
	l.DebugLog(logIdReferences, "ref definitions: compile final CEL environment")
	refProgress(3, 4, "compile CEL environment - final")
	env, err := cel.NewEnv(finalOpts...)
	if err != nil {
		return nil, err
	}
	refProgress(4, 4, "compile CEL environment - done")

	type refPickerCompiled struct {
		expr       cel.Expression
		tags       []string
		label      string
		attributes []types.RefAttribute
		reverse    bool
	}

	type refParser struct {
		predicate  cel.Predicate
		pickers    []refPickerCompiled
		tags       []string
		desc       string
		label      string
		attributes []types.RefAttribute
		reverse    bool
	}

	refParsers := []refParser{}

	allParsers := DefaultRefParsers()
	for _, extra := range extraParsers {
		allParsers = append(allParsers, extra...)
	}
	l.DebugLog(logIdReferences, "ref definitions: compile {count} ref parsers",
		log.Int("count", len(allParsers)))

	for i, parser := range allParsers {
		if len(allParsers) > 0 && (i%64 == 0 || i == len(allParsers)-1) {
			refProgress(i+1, len(allParsers), fmt.Sprintf("compile ref parsers - %d / %d", i+1, len(allParsers)))
		}
		var rp refParser
		predOrigin := fmt.Sprintf("embedded ref-parser #%d (%s)", i+1, parser.Desc)
		if parser.Cel == "" {
			rp.predicate, err = env.CompileSelectedPredicateAt(predOrigin, parser.Selector)
		} else {
			rp.predicate, err = env.CompileSelectedPredicateAt(predOrigin, parser.Selector, parser.Cel)
		}
		if err != nil {
			return nil, err
		}
		for pi, p := range parser.Pick {
			pickOrigin := fmt.Sprintf("%s · pick[%d]", predOrigin, pi)
			e, err := env.CompileExpressionAt(pickOrigin, p.Cel)
			if err != nil {
				return nil, err
			}
			rp.pickers = append(rp.pickers, refPickerCompiled{
				expr:       e,
				tags:       p.Tag,
				label:      p.Label,
				attributes: p.Attributes,
				reverse:    p.Reverse,
			})
		}
		rp.tags = parser.Tags
		rp.desc = parser.Desc
		rp.label = parser.Label
		rp.attributes = parser.Attributes
		rp.reverse = parser.Reverse
		refParsers = append(refParsers, rp)
	}

	var result []types.RefDefinition

	// execute – entity loop is outer so one lazy activation is shared across all parsers/pickers per entity
	l.DebugLog(logIdReferences, "ref definitions: execute {parsers} parsers over {entities} entities",
		log.Int("parsers", len(refParsers)),
		log.Int("entities", len(entities.Items)))
	for i, e := range entities.Items {
		if len(entities.Items) > 0 && (i%64 == 0 || i == len(entities.Items)-1) {
			refProgress(i+1, len(entities.Items), fmt.Sprintf("execute ref parsers - %d / %d", i+1, len(entities.Items)))
		}
		input := (&env).EntityActivation(e)
		fromId, err := e.Id()
		if err != nil {
			return nil, err
		}

		for _, rp := range refParsers {
			matches, err := rp.predicate.EvalBoolFromInput(input, types.MissingKeysReject)
			if err != nil {
				return nil, err
			}
			if !matches {
				continue
			}

			for _, pick := range rp.pickers {
				pickedRefVal, err := pick.expr.EvalFromInput(input)
				if errors.ErrKeyNotFound.MatchesError(err) {
					continue
				}
				if err != nil {
					return nil, err
				}

				lister, ok := pickedRefVal.(traits.Lister)
				if !ok {
					return nil, newPickerExprWantList(string(pick.expr.Expression()), pickedRefVal)
				}

				iter := lister.Iterator()
				for iter.HasNext() == celTypes.True {
					item := iter.Next()
					itemRefs, err := item.ConvertToNative(reflect.TypeFor[[]types.RefDefinition]())
					if err != nil {
						return nil, newPickerExprWantRefDefSlice(string(pick.expr.Expression()), item)
					}
					refDefs, ok := itemRefs.([]types.RefDefinition)
					if !ok {
						return nil, newPickerExprWantRefDefSlice(string(pick.expr.Expression()), itemRefs)
					}
					for _, picked := range refDefs {
						picked.Owner = fromId
						if picked.Type == "" {
							picked.Type = types.RefTypeDirect
						}
						if picked.Desc == "" && rp.desc != "" {
							picked.Desc = rp.desc
						}
						if pick.label != "" {
							picked.Label = pick.label
						} else if picked.Label == "" && rp.label != "" {
							picked.Label = rp.label
						}
						tagMerged := make(map[string]struct{}, len(picked.Tags)+len(rp.tags)+len(pick.tags))
						for _, t := range picked.Tags {
							tagMerged[t] = struct{}{}
						}
						for _, t := range rp.tags {
							tagMerged[t] = struct{}{}
						}
						for _, t := range pick.tags {
							tagMerged[t] = struct{}{}
						}
						if len(tagMerged) > 0 {
							allTags := make([]string, 0, len(tagMerged))
							for t := range tagMerged {
								allTags = append(allTags, t)
							}
							slices.Sort(allTags)
							picked.Tags = allTags
						}
						picked.Attributes = types.MergeRefAttributes(
							types.MergeRefAttributes(picked.Attributes, rp.attributes),
							pick.attributes,
						)
						picked.Reverse = picked.Reverse || rp.reverse || pick.reverse
						result = append(result, picked)
					}
				}
			}
		}
	}

	// Sort for deterministic output
	l.DebugLog(logIdReferences, "ref definitions: sort {count} ref definitions",
		log.Int("count", len(result)))
	refProgress(1, 1, fmt.Sprintf("sort ref definitions - %d", len(result)))
	slices.SortFunc(result, func(a, b types.RefDefinition) int {
		if c := cmp.Compare(a.Owner, b.Owner); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Direction, b.Direction); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Endpoint.Type, b.Endpoint.Type); c != 0 {
			return c
		}
		return cmp.Compare(a.Endpoint.Value, b.Endpoint.Value)
	})

	return result, nil
}

func materializeVirtualTargets(
	l log.Logger,
	entities entity.Entities,
	baseIDs sets.Set[types.Id],
	key types.EntityKeyUnstructured,
	refs []types.Ref,
	progress RefsProgress,
) (entity.Entities, bool, error) {
	refProgress := func(done int, total int, detail string) {
		if progress != nil {
			progress(done, total, detail)
		}
	}
	itemsByID := make(map[types.Id]entity.Entity, len(entities.Items))
	orderedIDs := make([]types.Id, 0, len(entities.Items))
	for i, item := range entities.Items {
		if len(entities.Items) > 0 && (i%256 == 0 || i == len(entities.Items)-1) {
			refProgress(i+1, len(entities.Items), fmt.Sprintf("virtual refs: index entities - %d / %d", i+1, len(entities.Items)))
		}
		id, err := item.Id()
		if err != nil {
			return entity.Entities{}, false, err
		}
		itemsByID[id] = item
		orderedIDs = append(orderedIDs, id)
	}

	changed := false
	materialized := 0
	for i, ref := range refs {
		if len(refs) > 0 && (i%256 == 0 || i == len(refs)-1) {
			refProgress(i+1, len(refs), fmt.Sprintf("virtual refs: scan refs - %d / %d", i+1, len(refs)))
			l.DebugLog(logIdReferences, "virtual refs: scanned refs",
				log.Int("done", i+1),
				log.Int("total", len(refs)),
				log.Int("materialized", materialized))
		}
		if !isMaterializedVirtualRef(ref) || baseIDs.Has(ref.To) {
			continue
		}
		sourceEntity, ok := itemsByID[ref.From]
		if !ok {
			continue
		}

		updated, wasNew, didChange, err := buildVirtualTargetEntity(itemsByID[ref.To], sourceEntity, ref, key)
		if err != nil {
			return entity.Entities{}, false, err
		}
		if !didChange {
			continue
		}
		itemsByID[ref.To] = updated
		if wasNew {
			orderedIDs = append(orderedIDs, ref.To)
		}
		materialized++
		changed = true
	}
	l.DebugLog(logIdReferences, "virtual refs: scan complete",
		log.Int("refs", len(refs)),
		log.Int("materialized", materialized),
		log.Bool("changed", changed))

	if !changed {
		return entities, false, nil
	}

	items := make([]entity.Entity, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		items = append(items, itemsByID[id])
	}
	expanded, err := entity.NewEntities(items)
	if err != nil {
		return entity.Entities{}, false, err
	}
	return expanded, true, nil
}

func isMaterializedVirtualRef(ref types.Ref) bool {
	return ref.EndpointType == types.RefEndpointTypeId && ref.RefMaterializesVirtualTarget()
}

func buildVirtualTargetEntity(
	existing entity.Entity,
	source entity.Entity,
	ref types.Ref,
	key types.EntityKeyUnstructured,
) (entity.Entity, bool, bool, error) {
	group, version, kind, ns, name, err := ref.To.Components()
	if err != nil {
		return entity.Entity{}, false, false, err
	}

	existingAppIDs := entityAppIDs(existing)
	newAppIDs := attributeAppIDs(ref.Attributes)
	mergedAppIDs := unionAppIDs(existingAppIDs, newAppIDs)

	keys := mergeStringKeys(
		existingVirtualDataKeys(existing, key),
		virtualDataKeysFromSource(source, key, kind, name),
	)

	builder := entity.NewEntityBuilder().
		WithGVK(types.NewGVK(group, version, kind)).
		WithName(name)
	if ns != "" {
		builder = builder.WithNamespace(ns).WithAppNamespace(types.AppNamespace(ns))
	}
	if len(mergedAppIDs) > 0 {
		builder = builder.WithAppIds(mergedAppIDs)
	}
	builder = builder.WithUnstructured(key, buildVirtualUnstructured(group, version, kind, ns, name, keys))

	updated, err := builder.Build()
	if err != nil {
		return entity.Entity{}, false, false, err
	}

	existingID, err := existing.Id()
	if err != nil || existingID == "" {
		return updated, true, true, nil
	}

	changed := len(existingAppIDs) != len(mergedAppIDs) || len(existingVirtualDataKeys(existing, key)) != len(keys)
	return updated, false, changed, nil
}

func entityAppIDs(item entity.Entity) []types.AppId {
	appIDs, err := item.AppIds()
	if err != nil {
		return nil
	}
	return appIDs
}

func attributeAppIDs(attrs []types.RefAttribute) []types.AppId {
	seen := sets.New[types.AppId]()
	for _, attr := range attrs {
		if attr.Type == types.RefAttributeOriginApp && attr.Value != "" {
			seen.Insert(types.AppId(attr.Value))
		}
	}
	result := seen.UnsortedList()
	slices.Sort(result)
	return result
}

func unionAppIDs(left []types.AppId, right []types.AppId) []types.AppId {
	seen := sets.New[types.AppId]()
	seen.Insert(left...)
	seen.Insert(right...)
	result := seen.UnsortedList()
	slices.Sort(result)
	return result
}

func existingVirtualDataKeys(item entity.Entity, key types.EntityKeyUnstructured) []string {
	u, ok := item.Unstructured(key)
	if !ok {
		return nil
	}
	switch gvk, err := item.GVKString(); {
	case err != nil:
		return nil
	case gvk == types.KubernetesGvkV1Secret:
		return secretDataKeys(u)
	case gvk == types.KubernetesGvkV1ConfigMap:
		return configMapDataKeys(u)
	default:
		return nil
	}
}

func virtualDataKeysFromSource(source entity.Entity, key types.EntityKeyUnstructured, targetKind types.Kind, targetName types.Name) []string {
	switch targetKind {
	case types.Kind("Secret"):
		if keys := secretTemplateKeys(source, key, targetName); len(keys) > 0 {
			return keys
		}
		u, ok := source.Unstructured(key)
		if !ok {
			return nil
		}
		return secretDataKeys(u)
	case types.Kind("ConfigMap"):
		u, ok := source.Unstructured(key)
		if !ok {
			return nil
		}
		return configMapDataKeys(u)
	default:
		return nil
	}
}

func secretTemplateKeys(source entity.Entity, key types.EntityKeyUnstructured, targetName types.Name) []string {
	u, ok := source.Unstructured(key)
	if !ok {
		return nil
	}
	templates, found, err := unstructured.NestedSlice(u.Object, "spec", "secretTemplates")
	if err != nil || !found {
		return nil
	}
	for _, templateValue := range templates {
		template, ok := templateValue.(map[string]any)
		if !ok {
			continue
		}
		if templateName, _ := template["name"].(string); templateName != string(targetName) {
			continue
		}
		keys := sets.New[string]()
		if data, ok := template["data"].(map[string]any); ok {
			for key := range data {
				keys.Insert(key)
			}
		}
		if stringData, ok := template["stringData"].(map[string]any); ok {
			for key := range stringData {
				keys.Insert(key)
			}
		}
		result := keys.UnsortedList()
		slices.Sort(result)
		return result
	}
	return nil
}

func secretDataKeys(u unstructured.Unstructured) []string {
	keys := sets.New[string]()
	if data, found, _ := unstructured.NestedStringMap(u.Object, "data"); found {
		for key := range data {
			keys.Insert(key)
		}
	}
	if stringData, found, _ := unstructured.NestedStringMap(u.Object, "stringData"); found {
		for key := range stringData {
			keys.Insert(key)
		}
	}
	result := keys.UnsortedList()
	slices.Sort(result)
	return result
}

func configMapDataKeys(u unstructured.Unstructured) []string {
	data, found, _ := unstructured.NestedStringMap(u.Object, "data")
	if !found {
		return nil
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func mergeStringKeys(left []string, right []string) []string {
	seen := sets.New[string]()
	seen.Insert(left...)
	seen.Insert(right...)
	result := seen.UnsortedList()
	slices.Sort(result)
	return result
}

func buildVirtualUnstructured(
	group types.Group,
	version types.Version,
	kind types.Kind,
	ns types.Namespace,
	name types.Name,
	keys []string,
) unstructured.Unstructured {
	apiVersion := string(version)
	if group != "" {
		apiVersion = string(group) + "/" + string(version)
	}
	metadata := map[string]any{"name": string(name)}
	metadata["annotations"] = map[string]any{
		"hydra-gitops.org/hydra/virtual": "true",
	}
	if ns != "" {
		metadata["namespace"] = string(ns)
	}
	object := map[string]any{
		"apiVersion": apiVersion,
		"kind":       string(kind),
		"metadata":   metadata,
	}
	switch kind {
	case types.Kind("Secret"):
		if len(keys) > 0 {
			stringData := make(map[string]any, len(keys))
			for _, key := range keys {
				stringData[key] = ""
			}
			object["stringData"] = stringData
		}
	case types.Kind("ConfigMap"):
		if len(keys) > 0 {
			data := make(map[string]any, len(keys))
			for _, key := range keys {
				data[key] = ""
			}
			object["data"] = data
		}
	}
	return unstructured.Unstructured{Object: object}
}
