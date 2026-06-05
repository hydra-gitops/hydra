package entity

import (
	"slices"

	"hydra-gitops.org/hydra/hydra-go/core/types"
)

func (entities Entities) Sort(orderProvider OrderProvider) (Entities, error) {
	sortedItems, err := sort(entities.Items, orderProvider.Order())
	if err != nil {
		return Entities{}, err
	}
	return NewEntities(sortedItems)
}

func sort(e []Entity, orders []Order) ([]Entity, error) {
	if len(orders) == 0 {
		return e, nil
	}

	order := orders[0]

	grouped, err := groupBy(e, order.Key)
	if err != nil {
		return nil, err
	}

	keys := []Key{}
	for key := range grouped {
		keys = append(keys, key)
	}

	slices.Sort(keys)
	if order.Direction() == types.DirectionDescending {
		slices.Reverse(keys)
	}

	result := []Entity{}
	for _, key := range keys {
		sorted, err := sort(grouped[key], orders[1:])
		if err != nil {
			return nil, err
		}
		result = append(result, sorted...)
	}

	return result, nil
}
