package references

import (
	"cmp"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// AnnotateRefsWithSource appends origin:source to each ref (merged with existing attributes).
func AnnotateRefsWithSource(refs []types.Ref, source string) []types.Ref {
	if len(refs) == 0 {
		return refs
	}
	out := make([]types.Ref, len(refs))
	for i := range refs {
		out[i] = refs[i]
		out[i].Attributes = types.MergeRefAttributes(refs[i].Attributes, []types.RefAttribute{
			{Type: types.RefAttributeOriginSource, Value: source},
		})
	}
	return out
}

// MergeRefLists merges ref slices by (From, To, RefType, EndpointType), unioning labels, tags, and attributes;
// reverse is OR'd; desc keeps first non-empty (no warning here).
func MergeRefLists(refsLists ...[]types.Ref) []types.Ref {
	type key struct {
		from         types.Id
		to           types.Id
		refType      types.RefType
		endpointType types.RefEndpointType
	}
	type acc struct {
		labels       sets.Set[string]
		tags         sets.Set[string]
		attributes   sets.Set[types.RefAttribute]
		desc         string
		reverse      bool
		endpointType types.RefEndpointType
		refType      types.RefType
	}
	m := make(map[key]*acc)
	for _, refs := range refsLists {
		for _, r := range refs {
			k := key{from: r.From, to: r.To, refType: r.RefType, endpointType: r.EndpointType}
			a, ok := m[k]
			if !ok {
				a = &acc{
					labels:       sets.New[string](),
					tags:         sets.New[string](),
					attributes:   sets.New[types.RefAttribute](),
					endpointType: r.EndpointType,
					refType:      r.RefType,
				}
				m[k] = a
			}
			a.labels.Insert(r.Labels...)
			a.tags.Insert(r.Tags...)
			for _, attr := range r.Attributes {
				a.attributes.Insert(attr)
			}
			if r.Desc != "" && a.desc == "" {
				a.desc = r.Desc
			}
			if r.Reverse {
				a.reverse = true
			}
		}
	}
	out := make([]types.Ref, 0, len(m))
	for k, a := range m {
		labels := a.labels.UnsortedList()
		slices.Sort(labels)
		tags := a.tags.UnsortedList()
		slices.Sort(tags)
		attributes := a.attributes.UnsortedList()
		slices.SortFunc(attributes, func(x, y types.RefAttribute) int {
			if c := cmp.Compare(x.Type, y.Type); c != 0 {
				return c
			}
			return cmp.Compare(x.Value, y.Value)
		})
		ref := types.Ref{
			RefType:      k.refType,
			EndpointType: k.endpointType,
			From:         k.from,
			To:           k.to,
			Reverse:      a.reverse,
		}
		if len(labels) > 0 {
			ref.Labels = labels
		}
		if len(tags) > 0 {
			ref.Tags = tags
		}
		if len(attributes) > 0 {
			ref.Attributes = attributes
		}
		if a.desc != "" {
			ref.Desc = a.desc
		}
		out = append(out, ref)
	}
	slices.SortFunc(out, func(a, b types.Ref) int {
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
	return out
}

// EnsureRefsHaveOriginSource adds origin:source when missing (used after enrichment that may add refs without the tag).
func EnsureRefsHaveOriginSource(refs []types.Ref, source string) []types.Ref {
	out := make([]types.Ref, len(refs))
	for i := range refs {
		if hasOriginSource(refs[i].Attributes) {
			out[i] = refs[i]
			continue
		}
		out[i] = refs[i]
		out[i].Attributes = types.MergeRefAttributes(refs[i].Attributes, []types.RefAttribute{
			{Type: types.RefAttributeOriginSource, Value: source},
		})
	}
	return out
}

func hasOriginSource(attrs []types.RefAttribute) bool {
	for _, a := range attrs {
		if a.Type == types.RefAttributeOriginSource {
			return true
		}
	}
	return false
}

// RefSourceForEntityKey returns RefSourceCluster for cluster unstructured keys, else RefSourceTemplate.
func RefSourceForEntityKey(key types.EntityKeyUnstructured) string {
	if key == types.KeyClusterEntity {
		return types.RefSourceCluster
	}
	return types.RefSourceTemplate
}
