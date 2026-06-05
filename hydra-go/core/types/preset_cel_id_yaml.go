package types

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// PresetCelItem is one CEL line in a cluster-defaults predicate. YAML: a string (Expr, required), or
// a mapping { cel: "...", optional: true }.
type PresetCelItem struct {
	Expr     string
	Optional bool
	Selector RefSelector
}

// UnmarshalYAML implements a scalar (plain CEL) or a mapping with optional selector fields.
func (c *PresetCelItem) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		if strings.TrimSpace(n.Value) == "" {
			return fmt.Errorf("cel item must not be empty")
		}
		c.Expr, c.Optional, c.Selector = n.Value, false, RefSelector{}
		return nil
	case yaml.MappingNode:
		var wire struct {
			Group      string `yaml:"group,omitempty"`
			Version    string `yaml:"version,omitempty"`
			Kind       string `yaml:"kind,omitempty"`
			ApiVersion string `yaml:"apiVersion,omitempty"`
			GVK        string `yaml:"gvk,omitempty"`
			Namespace  string `yaml:"namespace,omitempty"`
			GVKN       string `yaml:"gvkn,omitempty"`
			Name       string `yaml:"name,omitempty"`
			Id         string `yaml:"id,omitempty"`
			Cel        string `yaml:"cel"`
			Predicate  string `yaml:"predicate"`
			Optional   bool   `yaml:"optional"`
		}
		if err := n.Decode(&wire); err != nil {
			return err
		}
		selector, celExpr, err := RefSelectorInput{
			Group:      wire.Group,
			Version:    wire.Version,
			Kind:       wire.Kind,
			ApiVersion: wire.ApiVersion,
			GVK:        wire.GVK,
			Namespace:  wire.Namespace,
			GVKN:       wire.GVKN,
			Name:       wire.Name,
			Id:         wire.Id,
			Cel:        wire.Cel,
			Predicate:  wire.Predicate,
		}.Normalized()
		if err != nil {
			return err
		}
		c.Expr = string(celExpr)
		c.Optional = wire.Optional
		c.Selector = selector
		return nil
	default:
		return fmt.Errorf("cel item: expected string or map, got kind %v", n.Kind)
	}
}

// PresetCelList is the YAML "cel" sequence; each item is a string or { cel, optional }.
type PresetCelList []PresetCelItem

// UnmarshalYAML decodes a YAML sequence.
func (l *PresetCelList) UnmarshalYAML(n *yaml.Node) error {
	if l == nil {
		return fmt.Errorf("PresetCelList: nil receiver")
	}
	seq := n
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) != 1 {
			return fmt.Errorf("cel: document must have one root")
		}
		seq = n.Content[0]
	}
	// In embedded structs, the node is usually a sequence, not a document.
	if seq.Kind != yaml.SequenceNode {
		return fmt.Errorf("cel: must be a list, got %v", seq.Kind)
	}
	out := make(PresetCelList, 0, len(seq.Content))
	for i, ch := range seq.Content {
		var it PresetCelItem
		if err := ch.Decode(&it); err != nil {
			return fmt.Errorf("cel item %d: %w", i, err)
		}
		out = append(out, it)
	}
	*l = out
	return nil
}

// PresetIdItem is one explicit id in a predicate. YAML: a string (id), or { id: "...", optional: true }.
type PresetIdItem struct {
	Id       string
	Optional bool
}

// UnmarshalYAML supports a resource id string or a mapping { id, optional }.
func (c *PresetIdItem) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		if strings.TrimSpace(n.Value) == "" {
			return fmt.Errorf("id item must not be empty")
		}
		c.Id, c.Optional = n.Value, false
		return nil
	case yaml.MappingNode:
		var wire struct {
			Id       string `yaml:"id"`
			Optional bool   `yaml:"optional"`
		}
		if err := n.Decode(&wire); err != nil {
			return err
		}
		if strings.TrimSpace(wire.Id) == "" {
			return fmt.Errorf("id: mapping item must have non-empty id string")
		}
		c.Id, c.Optional = wire.Id, wire.Optional
		return nil
	default:
		return fmt.Errorf("id item: expected string or map, got kind %v", n.Kind)
	}
}

// PresetIdList is the YAML "ids" sequence.
type PresetIdList []PresetIdItem

// UnmarshalYAML decodes a YAML sequence.
func (l *PresetIdList) UnmarshalYAML(n *yaml.Node) error {
	if l == nil {
		return fmt.Errorf("PresetIdList: nil receiver")
	}
	seq := n
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) != 1 {
			return fmt.Errorf("ids: document must have one root")
		}
		seq = n.Content[0]
	}
	if seq.Kind != yaml.SequenceNode {
		return fmt.Errorf("ids: must be a list, got %v", seq.Kind)
	}
	out := make(PresetIdList, 0, len(seq.Content))
	for i, ch := range seq.Content {
		var it PresetIdItem
		if err := ch.Decode(&it); err != nil {
			return fmt.Errorf("id item %d: %w", i, err)
		}
		out = append(out, it)
	}
	*l = out
	return nil
}
