package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	selectors "hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type listSupport[T ~string] struct {
	name string
	list []T
}

var _ cel.SingletonLibrary = (*listSupport[selectors.Id])(nil)

func (support *listSupport[T]) LibraryName() string {
	return fmt.Sprintf("hydra.%s-support", support.name)
}

func (support *listSupport[T]) CompileOptions() []cel.EnvOption {
	result := []string{}
	for _, entry := range support.list {
		result = append(result, string(entry))
	}
	return []cel.EnvOption{
		cel.Constant(
			support.name,
			cel.ListType(cel.StringType),
			types.NewStringList(types.DefaultTypeAdapter, result),
		),
	}
}

func (*listSupport[T]) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

func ListSupport[T ~string](name string, list []T) cel.EnvOption {
	return cel.Lib(&listSupport[T]{name: name, list: list})
}

func SetSupport[T ~string](name string, set sets.Set[T]) cel.EnvOption {
	return cel.Lib(&listSupport[T]{name: name, list: set.UnsortedList()})
}
