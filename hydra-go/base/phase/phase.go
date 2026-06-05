package phase

import (
	"context"
	"fmt"
)

type Status string

const (
	StatusNext    Status = "next"
	StatusSkipped Status = "skipped"
	StatusAborted Status = "aborted"
)

type Result struct {
	Status Status
	Reason string
	Err    error
}

func Next() Result {
	return Result{Status: StatusNext}
}

func Skipped(reason string) Result {
	return Result{Status: StatusSkipped, Reason: reason}
}

func Aborted(err error) Result {
	return Result{Status: StatusAborted, Err: err}
}

type Func[T any] func(ctx context.Context, state *T) Result

type Item[T any] struct {
	Number      int
	Name        string
	Description string
	// WorkflowID is a stable phase identifier for logs and tests (often equal to Name).
	WorkflowID string
	Run        Func[T]
}

type Items[T any] struct {
	Items []Item[T]
}

type Builder[T any] struct {
	items []Item[T]
}

func NewBuilder[T any]() *Builder[T] {
	return &Builder[T]{}
}

func (b *Builder[T]) Add(name, description string, run Func[T]) *Builder[T] {
	b.items = append(b.items, Item[T]{
		Name:        name,
		Description: description,
		WorkflowID:  name,
		Run:         run,
	})
	return b
}

func (b *Builder[T]) Build() Items[T] {
	items := make([]Item[T], len(b.items))
	copy(items, b.items)
	for i := range items {
		items[i].Number = i + 1
		if items[i].WorkflowID == "" {
			items[i].WorkflowID = items[i].Name
		}
	}
	return Items[T]{Items: items}
}

type Report struct {
	Number      int
	Total       int
	Name        string
	Description string
	Status      Status
	Reason      string
	Err         error
}

func (r Report) Message() string {
	message := fmt.Sprintf("phase %d/%d: %s", r.Number, r.Total, r.Description)
	switch r.Status {
	case StatusSkipped:
		return message + " (skipped)"
	case StatusAborted:
		return message + " (aborted)"
	default:
		return message
	}
}

func (items Items[T]) Run(ctx context.Context, state *T, onReport func(Report)) error {
	total := len(items.Items)
	for _, item := range items.Items {
		result := item.Run(ctx, state)
		if result.Status == "" {
			result = Next()
		}

		report := Report{
			Number:      item.Number,
			Total:       total,
			Name:        item.Name,
			Description: item.Description,
			Status:      result.Status,
			Reason:      result.Reason,
			Err:         result.Err,
		}
		if onReport != nil {
			onReport(report)
		}
		if result.Status == StatusAborted && result.Err != nil {
			return result.Err
		}
	}
	return nil
}
