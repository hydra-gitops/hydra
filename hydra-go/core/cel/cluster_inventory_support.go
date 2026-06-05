package cel

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	goocel "github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	involvedEventMsgMaxRunes = 320
	involvedEventDisplayCap  = 10
)

type clusterInventorySupport struct {
	templateSnaps            []map[string]any
	clusterSnaps             []map[string]any
	live                     []entity.Entity
	liveKey                  types.EntityKeyUnstructured
	managedNamespaceEntities entity.Entities
}

var _ goocel.SingletonLibrary = (*clusterInventorySupport)(nil)

// entityMapFieldString reads a string field from maps produced by [Env.EntityToMap].
// Values may be plain Go strings or CEL ref.Vals (e.g. cel String).
func entityMapFieldString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if rv, ok := v.(ref.Val); ok {
		val := rv.Value()
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func entityMapInNamespace(m map[string]any, want string) bool {
	if want == "" {
		return true
	}
	gvk := entityMapFieldString(m, "gvk")
	name := entityMapFieldString(m, "name")
	ns := entityMapFieldString(m, "ns")
	if gvk == string(types.KubernetesGvkV1Namespace) && name == want {
		return true
	}
	return ns != "" && ns == want
}

func selectorMatchesEntityMap(selector types.RefSelector, m map[string]any) bool {
	if selector.Group != "" && entityMapFieldString(m, "group") != string(selector.Group) {
		return false
	}
	if selector.Version != "" && entityMapFieldString(m, "version") != string(selector.Version) {
		return false
	}
	if selector.Kind != "" && entityMapFieldString(m, "kind") != string(selector.Kind) {
		return false
	}
	if selector.Namespace != "" && !entityMapInNamespace(m, string(selector.Namespace)) {
		return false
	}
	if selector.Name != "" && entityMapFieldString(m, "name") != string(selector.Name) {
		return false
	}
	return true
}

func filterEntityMaps(maps []map[string]any, selector types.RefSelector) []map[string]any {
	if selector.IsZero() {
		return maps
	}
	out := make([]map[string]any, 0)
	for _, m := range maps {
		if selectorMatchesEntityMap(selector, m) {
			out = append(out, m)
		}
	}
	return out
}

func selectorFromObjectArg(arg ref.Val) (types.RefSelector, error) {
	var raw map[string]any
	switch v := arg.(type) {
	case traits.Mapper:
		native, err := v.ConvertToNative(reflect.TypeOf(map[string]any{}))
		if err != nil {
			return types.RefSelector{}, fmt.Errorf("expected selector object: %w", err)
		}
		var ok bool
		raw, ok = native.(map[string]any)
		if !ok {
			return types.RefSelector{}, fmt.Errorf("expected selector object")
		}
	default:
		var ok bool
		raw, ok = arg.Value().(map[string]any)
		if !ok {
			return types.RefSelector{}, fmt.Errorf("expected selector object")
		}
	}
	in := types.RefSelectorInput{}
	for key, value := range raw {
		s, err := selectorObjectString(value)
		if err != nil {
			return types.RefSelector{}, fmt.Errorf("selector field %q: %w", key, err)
		}
		switch key {
		case "group":
			in.Group = s
		case "version":
			in.Version = s
		case "kind":
			in.Kind = s
		case "apiVersion":
			in.ApiVersion = s
		case "gvk":
			in.GVK = s
		case "namespace", "ns":
			in.Namespace = s
		case "gvkn":
			in.GVKN = s
		case "name":
			in.Name = s
		case "id":
			in.Id = s
		case "cel":
			in.Cel = s
		case "predicate":
			in.Predicate = s
		}
	}
	selector, predicate, err := in.Normalized()
	if err != nil {
		return types.RefSelector{}, err
	}
	if strings.TrimSpace(string(predicate)) != "" {
		return types.RefSelector{}, fmt.Errorf("selector object does not support cel/predicate fields")
	}
	return selector, nil
}

func selectorObjectString(v any) (string, error) {
	if v == nil {
		return "", nil
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	if rv, ok := v.(ref.Val); ok {
		if s, ok := rv.Value().(string); ok {
			return s, nil
		}
	}
	return "", fmt.Errorf("expected string value")
}

func (c *clusterInventorySupport) LibraryName() string {
	return "hydra.cluster-inventory"
}

func (c *clusterInventorySupport) mapsToList(maps []map[string]any) ref.Val {
	elems := make([]any, len(maps))
	for i, m := range maps {
		elems[i] = m
	}
	return celtypes.NewDynamicList(celtypes.DefaultTypeAdapter, elems)
}

func (c *clusterInventorySupport) CompileOptions() []goocel.EnvOption {
	listOfMaps := goocel.ListType(goocel.MapType(goocel.StringType, goocel.DynType))
	selectorMap := goocel.MapType(goocel.StringType, goocel.DynType)
	return []goocel.EnvOption{
		goocel.Function("clusterEntities",
			goocel.Overload("clusterEntities_all",
				[]*goocel.Type{},
				listOfMaps,
				goocel.FunctionBinding(c.bindingClusterEntitiesAll),
			),
			goocel.Overload("clusterEntities_selector",
				[]*goocel.Type{selectorMap},
				listOfMaps,
				goocel.FunctionBinding(c.bindingClusterEntitiesSelector),
			),
		),
		goocel.Function("templateEntities",
			goocel.Overload("templateEntities_all",
				[]*goocel.Type{},
				listOfMaps,
				goocel.FunctionBinding(c.bindingTemplateEntitiesAll),
			),
			goocel.Overload("templateEntities_selector",
				[]*goocel.Type{selectorMap},
				listOfMaps,
				goocel.FunctionBinding(c.bindingTemplateEntitiesSelector),
			),
		),
		goocel.Function("entities",
			goocel.Overload("entities_all",
				[]*goocel.Type{},
				listOfMaps,
				goocel.FunctionBinding(c.bindingEntitiesAll),
			),
			goocel.Overload("entities_selector",
				[]*goocel.Type{selectorMap},
				listOfMaps,
				goocel.FunctionBinding(c.bindingEntitiesSelector),
			),
		),
		goocel.Function("managedNamespaces",
			goocel.Overload("managedNamespaces_all",
				[]*goocel.Type{},
				goocel.ListType(goocel.StringType),
				goocel.FunctionBinding(c.bindingManagedNamespaces),
			),
		),
		goocel.Function("involvedObjectEvents",
			goocel.Overload("involvedObjectEvents_int_string_string_string",
				[]*goocel.Type{goocel.IntType, goocel.StringType, goocel.StringType, goocel.StringType},
				goocel.ListType(goocel.StringType),
				goocel.FunctionBinding(c.bindingInvolvedObjectEvents),
			),
		),
	}
}

func (c *clusterInventorySupport) ProgramOptions() []goocel.ProgramOption {
	return nil
}

func (c *clusterInventorySupport) bindingClusterEntitiesAll(args ...ref.Val) ref.Val {
	if len(args) != 0 {
		return celtypes.NoSuchOverloadErr()
	}
	return c.mapsToList(c.clusterSnaps)
}

func (c *clusterInventorySupport) bindingClusterEntitiesSelector(args ...ref.Val) ref.Val {
	if len(args) != 1 {
		return celtypes.NoSuchOverloadErr()
	}
	selector, err := selectorFromObjectArg(args[0])
	if err != nil {
		return celtypes.NewErr("%s", err.Error())
	}
	return c.mapsToList(filterEntityMaps(c.clusterSnaps, selector))
}

func (c *clusterInventorySupport) bindingTemplateEntitiesAll(args ...ref.Val) ref.Val {
	if len(args) != 0 {
		return celtypes.NoSuchOverloadErr()
	}
	return c.mapsToList(c.templateSnaps)
}

func (c *clusterInventorySupport) bindingTemplateEntitiesSelector(args ...ref.Val) ref.Val {
	if len(args) != 1 {
		return celtypes.NoSuchOverloadErr()
	}
	selector, err := selectorFromObjectArg(args[0])
	if err != nil {
		return celtypes.NewErr("%s", err.Error())
	}
	return c.mapsToList(filterEntityMaps(c.templateSnaps, selector))
}

func (c *clusterInventorySupport) bindingEntitiesAll(args ...ref.Val) ref.Val {
	if len(args) != 0 {
		return celtypes.NoSuchOverloadErr()
	}
	n := len(c.templateSnaps) + len(c.clusterSnaps)
	elems := make([]any, 0, n)
	for _, m := range c.templateSnaps {
		elems = append(elems, m)
	}
	for _, m := range c.clusterSnaps {
		elems = append(elems, m)
	}
	return celtypes.NewDynamicList(celtypes.DefaultTypeAdapter, elems)
}

func (c *clusterInventorySupport) bindingEntitiesSelector(args ...ref.Val) ref.Val {
	if len(args) != 1 {
		return celtypes.NoSuchOverloadErr()
	}
	selector, err := selectorFromObjectArg(args[0])
	if err != nil {
		return celtypes.NewErr("%s", err.Error())
	}
	tmpl := filterEntityMaps(c.templateSnaps, selector)
	cl := filterEntityMaps(c.clusterSnaps, selector)
	elems := make([]any, 0, len(tmpl)+len(cl))
	for _, m := range tmpl {
		elems = append(elems, m)
	}
	for _, m := range cl {
		elems = append(elems, m)
	}
	return celtypes.NewDynamicList(celtypes.DefaultTypeAdapter, elems)
}

func (c *clusterInventorySupport) bindingManagedNamespaces(args ...ref.Val) ref.Val {
	if len(args) != 0 {
		return celtypes.NoSuchOverloadErr()
	}
	names := ManagedNamespaceNamesFromEntities(c.managedNamespaceEntities)
	return celtypes.NewStringList(celtypes.DefaultTypeAdapter, names)
}

func (c *clusterInventorySupport) bindingInvolvedObjectEvents(args ...ref.Val) ref.Val {
	if len(args) != 4 {
		return celtypes.NoSuchOverloadErr()
	}
	limitVal, ok := args[0].(celtypes.Int)
	if !ok {
		return celtypes.MaybeNoSuchOverloadErr(args[0])
	}
	limit := int(limitVal)
	if limit < 0 {
		limit = 0
	}
	if limit > involvedEventDisplayCap {
		limit = involvedEventDisplayCap
	}
	kindStr, ok := args[1].(celtypes.String)
	if !ok {
		return celtypes.MaybeNoSuchOverloadErr(args[1])
	}
	nameStr, ok := args[2].(celtypes.String)
	if !ok {
		return celtypes.MaybeNoSuchOverloadErr(args[2])
	}
	nsStr, ok := args[3].(celtypes.String)
	if !ok {
		return celtypes.MaybeNoSuchOverloadErr(args[3])
	}
	objectKind := string(kindStr)
	objectName := string(nameStr)
	objectNS := string(nsStr)

	lines := c.formatInvolvedObjectEvents(limit, objectKind, objectName, objectNS)
	return celtypes.NewStringList(celtypes.DefaultTypeAdapter, lines)
}

func (c *clusterInventorySupport) formatInvolvedObjectEvents(limit int, objectKind, objectName, objectNS string) []string {
	if limit <= 0 || objectKind == "" || objectName == "" {
		return nil
	}
	var collected []corev1.Event
	for _, e := range c.live {
		gvk, err := e.GVKString()
		if err != nil {
			continue
		}
		if gvk != types.KubernetesGvkV1Event && gvk != types.KubernetesGvkEventsK8sIoV1Event {
			continue
		}
		u, ok := e.Unstructured(c.liveKey)
		if !ok {
			continue
		}
		switch gvk {
		case types.KubernetesGvkV1Event:
			var ev corev1.Event
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &ev); err != nil {
				continue
			}
			if !involvedObjectMatches(ev.InvolvedObject, objectKind, objectName, objectNS) {
				continue
			}
			collected = append(collected, ev)
		case types.KubernetesGvkEventsK8sIoV1Event:
			var ev eventsv1.Event
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &ev); err != nil {
				continue
			}
			if !involvedObjectMatches(corev1.ObjectReference(ev.Regarding), objectKind, objectName, objectNS) {
				continue
			}
			collected = append(collected, eventsV1EventAsCoreV1(ev))
		}
	}
	if len(collected) == 0 {
		return nil
	}
	sort.SliceStable(collected, func(i, j int) bool {
		ti, tj := eventSortTime(collected[i]), eventSortTime(collected[j])
		if ti.Equal(tj) {
			return string(collected[i].UID) > string(collected[j].UID)
		}
		return ti.After(tj)
	})
	if len(collected) > limit {
		collected = collected[:limit]
	}
	out := make([]string, 0, len(collected))
	for _, ev := range collected {
		out = append(out, formatInvolvedEventLine(ev))
	}
	return out
}

// eventsV1EventAsCoreV1 maps an events.k8s.io/v1 Event into corev1.Event fields used by
// formatInvolvedEventLine / eventSortTime so involved-object display stays consistent.
func eventsV1EventAsCoreV1(e eventsv1.Event) corev1.Event {
	out := corev1.Event{
		ObjectMeta:     e.ObjectMeta,
		InvolvedObject: corev1.ObjectReference(e.Regarding),
		Reason:         e.Reason,
		Message:        e.Note,
		EventTime:      e.EventTime,
	}
	if e.Series != nil {
		out.Series = &corev1.EventSeries{
			Count:            e.Series.Count,
			LastObservedTime: e.Series.LastObservedTime,
		}
	}
	return out
}

func involvedObjectMatches(io corev1.ObjectReference, kind, name, ns string) bool {
	if io.Kind != kind || io.Name != name {
		return false
	}
	if ns == "" {
		return io.Namespace == ""
	}
	return io.Namespace == ns
}

func eventSortTime(e corev1.Event) time.Time {
	if !e.EventTime.IsZero() {
		return e.EventTime.Time
	}
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}
	return e.FirstTimestamp.Time
}

func formatInvolvedEventLine(e corev1.Event) string {
	typ := e.Type
	if typ == "" {
		typ = "Unknown"
	}
	reason := e.Reason
	if reason == "" {
		reason = "<no reason>"
	}
	msg := truncateRunesForEvent(e.Message, involvedEventMsgMaxRunes)
	c := eventLineCount(e)
	if c > 1 {
		return fmt.Sprintf("event: %s %s (x%d): %s", typ, reason, c, msg)
	}
	return fmt.Sprintf("event: %s %s: %s", typ, reason, msg)
}

func eventLineCount(e corev1.Event) int32 {
	if e.Series != nil && e.Series.Count > 0 {
		return e.Series.Count
	}
	return e.Count
}

func truncateRunesForEvent(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

// ClusterInventorySupport registers managedNamespaces(), templateEntities(), clusterEntities(), entities(),
// and involvedObjectEvents(...) for CEL environments. Snapshots must be produced with the same
// [Env.EntityToMap] as the final compiled environment.
//
// Template snapshots include entities that carry KeyTemplateEntity; cluster snapshots use KeyClusterEntity.
// clusterInventoryOverlay adds extra KeyClusterEntity snapshots to clusterEntities() only (for example
// the live cluster inventory during cluster review source parsing) without merging those entities into
// the live slice used for involvedObjectEvents. Rows are normalized with CopyItems(cluster→entity)
// so CEL `entity.*` paths match embedded parsers.
// managedNamespaces() is derived from managedNamespaceEntities when non-empty, otherwise from ents
// (same rules as the former HydraManagedNamespaces list).
func ClusterInventorySupport(env Env, ents entity.Entities, managedNamespaceEntities entity.Entities, clusterInventoryOverlay entity.Entities) (goocel.EnvOption, error) {
	clusterInventoryOverlaySnaps, err := ClusterInventoryOverlaySnapshots(env, clusterInventoryOverlay)
	if err != nil {
		return nil, err
	}
	return ClusterInventorySupportWithOverlaySnapshots(env, ents, managedNamespaceEntities, clusterInventoryOverlaySnaps)
}

func ClusterInventoryOverlaySnapshots(env Env, clusterInventoryOverlay entity.Entities) ([]map[string]any, error) {
	if clusterInventoryOverlay.Len() == 0 {
		return nil, nil
	}
	// Mirror entity.Entities.CopyItems(KeyClusterEntity → KeyEntity) like references.RefDefinitions
	// does for primary ents so CEL snapshots expose `entity.*` (e.g. workloadRegardingEvent filters)
	// for overlay-only cluster rows.
	normalizedOverlay, err := clusterInventoryOverlay.CopyItems(types.KeyClusterEntity, types.KeyEntity)
	if err != nil {
		return nil, err
	}
	clusterSnaps := make([]map[string]any, 0, normalizedOverlay.Len())
	for _, e := range normalizedOverlay.Items {
		if _, ok := e.Unstructured(types.KeyClusterEntity); ok {
			clusterSnaps = append(clusterSnaps, env.EntityToMap(e))
		}
	}
	return clusterSnaps, nil
}

func ClusterInventorySupportWithOverlaySnapshots(env Env, ents entity.Entities, managedNamespaceEntities entity.Entities, clusterInventoryOverlaySnaps []map[string]any) (goocel.EnvOption, error) {
	var templateSnaps, clusterSnaps []map[string]any
	var live []entity.Entity
	for _, e := range ents.Items {
		if _, ok := e.Unstructured(types.KeyTemplateEntity); ok {
			templateSnaps = append(templateSnaps, env.EntityToMap(e))
		}
		if _, ok := e.Unstructured(types.KeyClusterEntity); ok {
			clusterSnaps = append(clusterSnaps, env.EntityToMap(e))
			live = append(live, e)
		}
	}
	if len(clusterInventoryOverlaySnaps) > 0 {
		clusterSnaps = append(clusterSnaps, clusterInventoryOverlaySnaps...)
	}
	managedSrc := managedNamespaceEntities
	if managedSrc.Len() == 0 {
		managedSrc = ents
	}
	return goocel.Lib(&clusterInventorySupport{
		templateSnaps:            templateSnaps,
		clusterSnaps:             clusterSnaps,
		live:                     live,
		liveKey:                  types.KeyClusterEntity,
		managedNamespaceEntities: managedSrc,
	}), nil
}
