package types

import (
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Value interface {
	Value() any
	DeepCopy() Value
}

type ValueString struct {
	v string
}

func (v ValueString) Value() any {
	return v.String()
}

func (v ValueString) String() string {
	return v.v
}

func (v ValueString) DeepCopy() Value {
	return NewValueString(v.v)
}

func NewValueString(value string) Value {
	return ValueString{v: value}
}

type ValueBool struct {
	v bool
}

func (v ValueBool) Value() any {
	return v.Bool()
}

func (v ValueBool) Bool() bool {
	return v.v
}

func (v ValueBool) DeepCopy() Value {
	return NewValueBool(v.v)
}

func NewValueBool(value bool) Value {
	return ValueBool{v: value}
}

type ValueSlice[T any] struct {
	v []T
}

func (v ValueSlice[T]) Value() any {
	return v.Slice()
}

func (v ValueSlice[T]) Slice() []T {
	return slices.Clone(v.v)
}

func (v ValueSlice[T]) DeepCopy() Value {
	return NewValueSlice(v.v)
}

func NewValueSlice[T any](value []T) Value {
	return ValueSlice[T]{v: slices.Clone(value)}
}

type ValueInt struct {
	v int
}

func (v ValueInt) Value() any {
	return v.Int()
}

func (v ValueInt) Int() int {
	return v.v
}

func (v ValueInt) DeepCopy() Value {
	return NewValueInt(v.v)
}

func NewValueInt(value int) Value {
	return ValueInt{v: value}
}

type ValueUnstructured struct {
	v unstructured.Unstructured
}

func (v ValueUnstructured) Value() any {
	return v.Unstructured()
}

func (v ValueUnstructured) RawUnstructured() unstructured.Unstructured {
	return v.v
}

func (v ValueUnstructured) Unstructured() unstructured.Unstructured {
	return *v.v.DeepCopy()
}

func (v ValueUnstructured) DeepCopy() Value {
	return NewValueUnstructured(v.v)
}

func NewValueUnstructured(value unstructured.Unstructured) Value {
	return ValueUnstructured{v: *value.DeepCopy()}
}
