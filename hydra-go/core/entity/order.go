package entity

import (
	"slices"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type Key string

type OrderProvider interface {
	Order() []Order
}

type Order interface {
	Direction() types.Direction
	Key(e Entity) (Key, error)
}

type FieldOrder[T ~string] struct {
	direction types.Direction
	field     func(e Entity) (T, error)
}

var _ OrderProvider = FieldOrder[types.Id]{}
var _ Order = FieldOrder[types.Id]{}

func (o FieldOrder[T]) Order() []Order {
	return []Order{o}
}

func (o FieldOrder[T]) Direction() types.Direction {
	return o.direction
}

func (o FieldOrder[T]) Key(e Entity) (Key, error) {
	id, err := o.field(e)
	if err != nil {
		return "", err
	}
	return Key(id), nil
}

func NewFieldOrder[T ~string](
	direction types.Direction,
	field func(e Entity) (T, error),
) FieldOrder[T] {
	return FieldOrder[T]{
		direction: direction,
		field:     field,
	}
}

func NewIdFieldOrder(direction types.Direction) FieldOrder[types.Id] {
	return NewFieldOrder(direction, func(e Entity) (types.Id, error) {
		return e.Id()
	})
}

type ReverseOrder struct {
	order Order
}

var _ OrderProvider = ReverseOrder{}
var _ Order = ReverseOrder{}

func (o ReverseOrder) Order() []Order {
	if orderProvider, ok := o.order.(OrderProvider); ok {
		return append([]Order{o}, orderProvider.Order()...)
	}
	return append([]Order{o}, o.order)
}

func (o ReverseOrder) Direction() types.Direction {
	if o.order.Direction() == types.DirectionAscending {
		return types.DirectionDescending
	}
	return types.DirectionAscending
}

func (o ReverseOrder) Key(e Entity) (Key, error) {
	return "", nil
}

func NewReverseOrder(order Order) ReverseOrder {
	return ReverseOrder{
		order: order,
	}
}

type OrderFunc struct {
	direction types.Direction
	order     func(e Entity) (string, error)
}

var _ OrderProvider = OrderFunc{}
var _ Order = OrderFunc{}

func (o OrderFunc) Order() []Order {
	return []Order{o}
}

func (o OrderFunc) Direction() types.Direction {
	return o.direction
}

func (o OrderFunc) Key(e Entity) (Key, error) {
	result, err := o.order(e)
	if err != nil {
		return "", err
	}
	return Key(result), nil
}

func NewOrderFunc(
	direction types.Direction,
	order func(e Entity) (string, error),
) OrderFunc {
	return OrderFunc{
		direction: direction,
		order:     order,
	}
}

func NewFieldSetOrder[T ~string](
	direction types.Direction,
	field func(e Entity) (T, error),
	values ...T,
) (OrderProvider, error) {
	if len(values) == 0 {
		return nil, log.CreateError(errors.ErrInternalError, "no values provided for field set order")
	}

	return NewOrderFunc(direction, func(e Entity) (string, error) {
		value, err := field(e)
		if err != nil {
			return "", err
		}
		if slices.Contains(values, value) {
			return "0", nil
		}
		return "1", nil
	}), nil
}

type ChainOrder struct {
	order OrderProvider
	next  *ChainOrder
}

var _ OrderProvider = ChainOrder{}

func NewChainOrder(orders ...OrderProvider) (ChainOrder, error) {
	switch len(orders) {
	case 0:
		return ChainOrder{}, log.CreateError(errors.ErrInternalError, "no orders provided for chain order")
	case 1:
		return ChainOrder{order: orders[0]}, nil
	default:
		next, _ := NewChainOrder(orders[1:]...)
		return ChainOrder{order: orders[0], next: &next}, nil
	}
}

func (o ChainOrder) Order() []Order {
	chain := o.order.Order()

	if o.next != nil {
		chain = append(chain, o.next.Order()...)
	}

	return chain
}
