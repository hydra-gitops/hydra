package tui

import (
	"cmp"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type FilterField string

const (
	FilterFieldID        FilterField = "id"
	FilterFieldName      FilterField = "name"
	FilterFieldGVK       FilterField = "gvk"
	FilterFieldGVKN      FilterField = "gvkn"
	FilterFieldGroup     FilterField = "group"
	FilterFieldVersion   FilterField = "version"
	FilterFieldKind      FilterField = "kind"
	FilterFieldNamespace FilterField = "namespace"
	FilterFieldRelation  FilterField = "relation"
	FilterFieldStatus    FilterField = "status"
)

var pickerFilterFields = []FilterField{
	FilterFieldID,
	FilterFieldGVK,
	FilterFieldNamespace,
	FilterFieldKind,
	FilterFieldName,
	FilterFieldGroup,
	FilterFieldVersion,
	FilterFieldGVKN,
}

var refTreeFilterFields = []FilterField{
	FilterFieldID,
	FilterFieldGVK,
	FilterFieldNamespace,
	FilterFieldKind,
	FilterFieldName,
	FilterFieldGroup,
	FilterFieldVersion,
	FilterFieldGVKN,
	FilterFieldRelation,
	FilterFieldStatus,
}

type filterPopupFocus int

const (
	filterPopupFocusQuery filterPopupFocus = iota
	filterPopupFocusField
)

type pickerSortField string

const (
	pickerSortID        pickerSortField = "id"
	pickerSortKind      pickerSortField = "kind"
	pickerSortNamespace pickerSortField = "namespace"
	pickerSortName      pickerSortField = "name"
)

var pickerSortFields = []pickerSortField{
	pickerSortID,
	pickerSortKind,
	pickerSortNamespace,
	pickerSortName,
}

type refTreeSortField string

const (
	refTreeSortDist     refTreeSortField = "dist"
	refTreeSortID       refTreeSortField = "id"
	refTreeSortRelation refTreeSortField = "relation"
	refTreeSortStatus   refTreeSortField = "status"
)

// Order matches left-to-right list column headers: Dist, Ref id, Relation, Status.
var refTreeSortFields = []refTreeSortField{
	refTreeSortDist,
	refTreeSortID,
	refTreeSortRelation,
	refTreeSortStatus,
}

func nextPickerSortField(cur pickerSortField) pickerSortField {
	return pickerSortFields[nextIndexInSlice(pickerSortFields, cur)]
}

func nextRefTreeSortField(cur refTreeSortField) refTreeSortField {
	return refTreeSortFields[nextIndexInSlice(refTreeSortFields, cur)]
}

func prevPickerSortField(cur pickerSortField) pickerSortField {
	return pickerSortFields[prevIndexInSlice(pickerSortFields, cur)]
}

func prevRefTreeSortField(cur refTreeSortField) refTreeSortField {
	return refTreeSortFields[prevIndexInSlice(refTreeSortFields, cur)]
}

func prevIndexInSlice[T comparable](items []T, cur T) int {
	if len(items) == 0 {
		return 0
	}
	idx := slices.Index(items, cur)
	if idx < 0 {
		idx = 0
	}
	return (idx - 1 + len(items)) % len(items)
}

func nextFilterField(fields []FilterField, cur FilterField, dir int) FilterField {
	if len(fields) == 0 {
		return FilterFieldID
	}
	idx := slices.Index(fields, cur)
	if idx < 0 {
		idx = 0
	}
	if dir < 0 {
		idx = (idx - 1 + len(fields)) % len(fields)
	} else {
		idx = (idx + 1) % len(fields)
	}
	return fields[idx]
}

func nextIndexInSlice[T comparable](items []T, cur T) int {
	if len(items) == 0 {
		return 0
	}
	idx := slices.Index(items, cur)
	if idx < 0 {
		return 0
	}
	return (idx + 1) % len(items)
}

func filterQueryMatch(value, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), q)
}

func idFieldValue(id types.Id, field FilterField) string {
	group, version, kind, namespace, name, err := id.Components()
	if err != nil {
		if field == FilterFieldID {
			return string(id)
		}
		return ""
	}
	gvk := idGVKString(id)
	switch field {
	case FilterFieldID:
		return string(id)
	case FilterFieldName:
		return string(name)
	case FilterFieldGVK:
		return gvk
	case FilterFieldGVKN:
		s, gerr := id.GVKNString()
		if gerr != nil {
			return ""
		}
		return s
	case FilterFieldGroup:
		return string(group)
	case FilterFieldVersion:
		return string(version)
	case FilterFieldKind:
		return string(kind)
	case FilterFieldNamespace:
		return string(namespace)
	default:
		return ""
	}
}

func refRowFieldValue(row RefEdgeRow, field FilterField) string {
	switch field {
	case FilterFieldRelation:
		return row.Relation
	case FilterFieldStatus:
		if row.IsSelf {
			return ""
		}
		return clusterRefEdgeStatus(row.Ref)
	default:
		return idFieldValue(row.OtherID, field)
	}
}

func compareLower(a, b string) int {
	return cmp.Compare(strings.ToLower(a), strings.ToLower(b))
}
