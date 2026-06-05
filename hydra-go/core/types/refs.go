package types

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type RefSelectorInput struct {
	Group      string `yaml:"group,omitempty"`
	Version    string `yaml:"version,omitempty"`
	Kind       string `yaml:"kind,omitempty"`
	ApiVersion string `yaml:"apiVersion,omitempty"`
	GVK        string `yaml:"gvk,omitempty"`
	Namespace  string `yaml:"namespace,omitempty"`
	GVKN       string `yaml:"gvkn,omitempty"`
	Name       string `yaml:"name,omitempty"`
	Id         string `yaml:"id,omitempty"`
	Cel        string `yaml:"cel,omitempty"`
	Predicate  string `yaml:"predicate,omitempty"`
}

type RefSelector struct {
	Group     Group
	Version   Version
	Kind      Kind
	Namespace Namespace
	Name      Name
}

func (s RefSelector) IsZero() bool {
	return s.Group == "" && s.Version == "" && s.Kind == "" && s.Namespace == "" && s.Name == ""
}

func (s RefSelector) CelPredicate() CelPredicate {
	parts := make([]string, 0, 5)
	if s.Group != "" {
		parts = append(parts, fmt.Sprintf(`group == %q`, s.Group))
	}
	if s.Version != "" {
		parts = append(parts, fmt.Sprintf(`version == %q`, s.Version))
	}
	if s.Kind != "" {
		parts = append(parts, fmt.Sprintf(`kind == %q`, s.Kind))
	}
	if s.Namespace != "" {
		parts = append(parts, fmt.Sprintf(`ns == %q`, s.Namespace))
	}
	if s.Name != "" {
		parts = append(parts, fmt.Sprintf(`name == %q`, s.Name))
	}
	return CelPredicate(strings.Join(parts, " && "))
}

func (s RefSelector) MatchPredicate(extra CelPredicate) CelPredicate {
	selector := strings.TrimSpace(string(s.CelPredicate()))
	rest := strings.TrimSpace(string(extra))
	switch {
	case selector == "":
		return CelPredicate(rest)
	case rest == "":
		return CelPredicate(selector)
	default:
		return CelPredicate("(" + selector + ") && (" + rest + ")")
	}
}

func (in RefSelectorInput) Normalized() (RefSelector, CelPredicate, error) {
	cel := strings.TrimSpace(in.Cel)
	predicate := strings.TrimSpace(in.Predicate)
	if cel != "" && predicate != "" {
		return RefSelector{}, "", fmt.Errorf("cel and predicate cannot both be set")
	}
	if cel == "" {
		cel = predicate
	}

	var selector RefSelector
	if err := setSelectorComponent("group", &selector.Group, Group(strings.TrimSpace(in.Group)), "group"); err != nil {
		return RefSelector{}, "", err
	}
	if err := setSelectorComponent("version", &selector.Version, Version(strings.TrimSpace(in.Version)), "version"); err != nil {
		return RefSelector{}, "", err
	}
	if err := setSelectorComponent("kind", &selector.Kind, Kind(strings.TrimSpace(in.Kind)), "kind"); err != nil {
		return RefSelector{}, "", err
	}
	if err := setSelectorComponent("namespace", &selector.Namespace, Namespace(strings.TrimSpace(in.Namespace)), "namespace"); err != nil {
		return RefSelector{}, "", err
	}
	if err := setSelectorComponent("name", &selector.Name, Name(strings.TrimSpace(in.Name)), "name"); err != nil {
		return RefSelector{}, "", err
	}
	if err := applyApiVersionInput(&selector, strings.TrimSpace(in.ApiVersion), "apiVersion"); err != nil {
		return RefSelector{}, "", err
	}
	if err := applyGVKInput(&selector, strings.TrimSpace(in.GVK), "gvk"); err != nil {
		return RefSelector{}, "", err
	}
	if err := applyGVKNInput(&selector, strings.TrimSpace(in.GVKN), "gvkn"); err != nil {
		return RefSelector{}, "", err
	}
	if err := applyIDInput(&selector, strings.TrimSpace(in.Id), "id"); err != nil {
		return RefSelector{}, "", err
	}

	if selector.IsZero() && cel == "" {
		return RefSelector{}, "", fmt.Errorf("at least one selector field or cel must be set")
	}

	return selector, CelPredicate(cel), nil
}

func setSelectorComponent[T ~string](field string, target *T, value T, source string) error {
	if value == "" {
		return nil
	}
	if *target != "" && *target != value {
		return fmt.Errorf("%s conflicts with %s: %q != %q", source, field, *target, value)
	}
	*target = value
	return nil
}

func applyApiVersionInput(selector *RefSelector, raw string, source string) error {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "/")
	switch len(parts) {
	case 1:
		return setSelectorComponent("version", &selector.Version, Version(parts[0]), source)
	case 2:
		if err := setSelectorComponent("group", &selector.Group, Group(parts[0]), source); err != nil {
			return err
		}
		return setSelectorComponent("version", &selector.Version, Version(parts[1]), source)
	default:
		return fmt.Errorf("%s must be version or group/version, got %q", source, raw)
	}
}

func applyGVKInput(selector *RefSelector, raw string, source string) error {
	if raw == "" {
		return nil
	}
	group, version, kind, err := GVKString(raw).Components()
	if err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}
	if err := setSelectorComponent("group", &selector.Group, group, source); err != nil {
		return err
	}
	if err := setSelectorComponent("version", &selector.Version, version, source); err != nil {
		return err
	}
	return setSelectorComponent("kind", &selector.Kind, kind, source)
}

func applyGVKNInput(selector *RefSelector, raw string, source string) error {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "/")
	switch len(parts) {
	case 2:
		return applyGVKInput(selector, raw, source)
	case 3:
		if looksLikeVersion(parts[1]) {
			return applyGVKInput(selector, raw, source)
		}
		if err := applyGVKInput(selector, strings.Join(parts[:2], "/"), source); err != nil {
			return err
		}
		return setSelectorComponent("namespace", &selector.Namespace, Namespace(parts[2]), source)
	case 4:
		if err := applyGVKInput(selector, strings.Join(parts[:3], "/"), source); err != nil {
			return err
		}
		return setSelectorComponent("namespace", &selector.Namespace, Namespace(parts[3]), source)
	default:
		return fmt.Errorf("%s must be gvk or gvk/namespace, got %q", source, raw)
	}
}

func applyIDInput(selector *RefSelector, raw string, source string) error {
	if raw == "" {
		return nil
	}
	group, version, kind, namespace, name, err := Id(raw).Components()
	if err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}
	if err := setSelectorComponent("group", &selector.Group, group, source); err != nil {
		return err
	}
	if err := setSelectorComponent("version", &selector.Version, version, source); err != nil {
		return err
	}
	if err := setSelectorComponent("kind", &selector.Kind, kind, source); err != nil {
		return err
	}
	if err := setSelectorComponent("namespace", &selector.Namespace, namespace, source); err != nil {
		return err
	}
	return setSelectorComponent("name", &selector.Name, name, source)
}

func looksLikeVersion(part string) bool {
	if len(part) < 2 || part[0] != 'v' {
		return false
	}
	return part[1] >= '0' && part[1] <= '9'
}

// RefPicker is one entry under ref-parser `pick`: a CEL expression plus optional metadata
// that applies only to refs produced by this pick (merged with parser-level fields).
type RefPicker struct {
	Cel        CelExpression
	Tag        []string
	Label      string
	Attributes []RefAttribute
	Reverse    bool
}

type RefParser struct {
	Selector   RefSelector
	Cel        CelPredicate
	Pick       []RefPicker
	Tags       []string
	Desc       string
	Label      string
	Attributes []RefAttribute
	Reverse    bool
}

func (p RefParser) MatchPredicate() CelPredicate {
	return p.Selector.MatchPredicate(p.Cel)
}

type RefType string

const (
	RefTypeDirect    RefType = "direct"
	RefTypeIndirect  RefType = "indirect"
	RefTypeRuntime   RefType = "runtime"
	RefTypeRegarding RefType = "regarding"
)

// RefTagOptionalStartup marks refs that participate in optional startup/scale ordering
// (operator/monitoring dependencies, and synthetic transitive workload edges).
const RefTagOptionalStartup = "optional:startup"

// RefTagOptionalRef marks refs derived from Kubernetes fields that set optional: true
// (env valueFrom, envFrom, volume sources, projected volume sources).
const RefTagOptionalRef = "optional:ref"

// RefTagBootstrapGuard marks ref-parser groups used by cluster apply to detect
// bootstrap-critical resources (see global.hydra.refs tag bootstrap-guard).
const RefTagBootstrapGuard = "bootstrap-guard"

// RefTagRuntime marks uninstall / uninstall-force / backup ref groups whose ownership predicates apply only
// when the resource id appears in no standalone per-app template render (cluster-only ids).
// Hydra never evaluates [runtime] groups for ownership when the id is template-mapped: matching
// uses the same predicate set as hydra local review (without [runtime] groups). For cluster-only
// ids, hydra gitops review and cluster uninstall include [runtime] groups alongside non-runtime
// uninstall rules.
const RefTagRuntime = "runtime"

// RefLabelNamespace is the ref label for synthetic edges from a namespaced entity to its Namespace.
const RefLabelNamespace = "namespace"

const (
	RefGeneratedJob        = "job"
	RefGeneratedController = "controller"
)

type RefEndpointType string

const (
	RefEndpointTypeId       RefEndpointType = "id"
	RefEndpointTypeProvider RefEndpointType = "provider"
)

type RefEndpoint struct {
	Type  RefEndpointType `yaml:"type"`
	Value string          `yaml:"value"`
}

func (re RefEndpoint) Id() (Id, bool) {
	if re.Type != RefEndpointTypeId {
		return "", false
	}
	return Id(re.Value), true
}

type RefAttribute struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

func (ra RefAttribute) MarshalYAML() (any, error) {
	return RefParserAttribute{ra.Type: ra.Value}, nil
}

func (ra *RefAttribute) UnmarshalYAML(value *yaml.Node) error {
	var parserAttr RefParserAttribute
	if err := value.Decode(&parserAttr); err != nil {
		return err
	}
	if len(parserAttr) == 1 {
		for key, val := range parserAttr {
			ra.Type = key
			ra.Value = val
			return nil
		}
	}
	return fmt.Errorf("attribute must contain exactly one key/value pair")
}

type RefParserAttribute map[string]string

const (
	RefAttributeOriginApp       = "origin:app"
	RefAttributeOriginWorkload  = "origin:workload"
	RefAttributeOriginGenerated = "origin:generated"
	// RefAttributeOriginOwner marks refs derived from Kubernetes metadata.ownerReferences.
	RefAttributeOriginOwner = "origin:owner"
	// RefAttributeOriginObjectset marks refs derived from Rancher wrangler objectset.rio.cattle.io
	// owner annotations (objectset.rio.cattle.io/owner-gvk|owner-name|owner-namespace).
	RefAttributeOriginObjectset = "origin:objectset"
	// RefAttributeRegarding marks events.k8s.io/v1 Event -> subject edges; value is the canonical Hydra Id string (same as ref.To).
	RefAttributeRegarding = "regarding"
)

// Values for RefAttributeOriginOwner (Kubernetes owner reference role).
const (
	RefOwnerRoleController = "controller"
	RefOwnerRoleDependent  = "dependent"
)

// Kubernetes ownerReferences metadata (see ref-parsers/kubernetes/_all.yaml).
const (
	RefAttributeKubernetesOwnerController    = "kubernetes:ownerController"
	RefAttributeKubernetesBlockOwnerDeletion = "kubernetes:blockOwnerDeletion"
)

// RefAttributeOriginSource records which entity set produced a ref edge (template render vs live cluster).
const RefAttributeOriginSource = "origin:source"

// Generic ref attributes that mark refs as parent-resolution fallbacks for workload closure /
// preset owner traversal. Concrete via values live in YAML/test data, not in Go code.
const (
	RefAttributeHydraParentVia       = "hydra:parent-via"
	RefAttributeHydraParentDirection = "hydra:parent-direction"
)

const (
	RefSourceTemplate = "template"
	RefSourceCluster  = "cluster"
	// RefSourceTest marks refs produced by unit tests and chart ref test harnesses (not live API inventory).
	RefSourceTest = "test"
)

func RefAttributesFromParserAttributes(attrs []RefParserAttribute) ([]RefAttribute, error) {
	result := make([]RefAttribute, 0, len(attrs))
	for i, attr := range attrs {
		if len(attr) != 1 {
			return nil, fmt.Errorf("attributes[%d] must contain exactly one key/value pair", i)
		}
		for key, value := range attr {
			if key == "generated" {
				return nil, fmt.Errorf("attributes[%d] uses unsupported legacy key %q; use %q", i, key, RefAttributeOriginGenerated)
			}
			result = append(result, RefAttribute{Type: key, Value: value})
		}
	}
	return result, nil
}

func MergeRefAttributes(existing []RefAttribute, additional []RefAttribute) []RefAttribute {
	if len(additional) == 0 {
		return existing
	}
	merged := make(map[RefAttribute]struct{}, len(existing)+len(additional))
	for _, attr := range existing {
		merged[attr] = struct{}{}
	}
	for _, attr := range additional {
		merged[attr] = struct{}{}
	}
	result := make([]RefAttribute, 0, len(merged))
	for attr := range merged {
		result = append(result, attr)
	}
	slices.SortFunc(result, func(a, b RefAttribute) int {
		if c := cmp.Compare(a.Type, b.Type); c != 0 {
			return c
		}
		return cmp.Compare(a.Value, b.Value)
	})
	return result
}

// Ref represents a reference between two entities
type Ref struct {
	RefType      RefType         `yaml:"refType"`
	EndpointType RefEndpointType `yaml:"endpointType"`
	From         Id              `yaml:"from"`
	To           Id              `yaml:"to"`
	Labels       []string        `yaml:"labels,omitempty"`
	Tags         []string        `yaml:"tags,omitempty"`
	Desc         string          `yaml:"desc,omitempty"`
	Attributes   []RefAttribute  `yaml:"attributes,omitempty"`
	Reverse      bool            `yaml:"reverse,omitempty"`
}

func (r Ref) HasTag(tag string) bool {
	for _, existing := range r.Tags {
		if existing == tag {
			return true
		}
	}
	return false
}

// HasGeneratedValue reports whether the ref carries origin:generated:<value> (see RefAttributeOriginGenerated).
func (r Ref) HasGeneratedValue(value string) bool {
	for _, a := range r.Attributes {
		if a.Type == RefAttributeOriginGenerated && a.Value == value {
			return true
		}
	}
	return false
}

// RefMaterializesVirtualTarget reports whether the ref declares a virtual target materialized at
// apply/runtime (origin:generated job or controller).
func (r Ref) RefMaterializesVirtualTarget() bool {
	return r.HasGeneratedValue(RefGeneratedJob) || r.HasGeneratedValue(RefGeneratedController)
}

type RefDirection string

const (
	RefDirectionIncoming RefDirection = "incoming"
	RefDirectionOutgoing RefDirection = "outgoing"
)

// RefDefinition represents a reference endpoint for an entity
type RefDefinition struct {
	Owner      Id             `yaml:"owner"`
	Type       RefType        `yaml:"type"`
	Direction  RefDirection   `yaml:"direction"`
	Endpoint   RefEndpoint    `yaml:"endpoint"`
	Label      string         `yaml:"label,omitempty"`
	Tags       []string       `yaml:"tags,omitempty"`
	Desc       string         `yaml:"desc,omitempty"`
	Attributes []RefAttribute `yaml:"attributes,omitempty"`
	Reverse    bool           `yaml:"reverse,omitempty"`
}
