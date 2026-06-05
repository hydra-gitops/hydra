package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// ServiceInfo holds the information needed for label matching
type ServiceInfo struct {
	Id       string            // "namespace/name"
	Selector map[string]string // spec.selector labels
}

type serviceSupport struct {
	// services grouped by namespace
	servicesByNamespace map[string][]ServiceInfo
}

var _ cel.SingletonLibrary = (*serviceSupport)(nil)

func (support *serviceSupport) LibraryName() string {
	return "hydra.service-support"
}

func (support *serviceSupport) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// matchingServices(namespace, podLabels) returns list of service IDs (ns/name) that match
		cel.Function(
			"matchingServices",
			cel.Overload(
				"matchingServices_string_map",
				[]*cel.Type{
					cel.StringType, // namespace
					cel.MapType(cel.StringType, cel.StringType), // pod labels
				},
				cel.ListType(cel.StringType), // list of service IDs
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					ns, ok := args[0].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}

					podLabels, ok := args[1].(traits.Mapper)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}

					// Get services for this namespace
					services, exists := support.servicesByNamespace[string(ns)]
					if !exists {
						return types.NewStringList(types.DefaultTypeAdapter, []string{})
					}

					var matching []string
					for _, svc := range services {
						if len(svc.Selector) == 0 {
							// Services without selector don't select any pods
							continue
						}

						// Check if all selector labels match pod labels
						matches := true
						for k, v := range svc.Selector {
							podVal, found := podLabels.Find(types.String(k))
							if !found {
								matches = false
								break
							}
							if podVal.Value() != v {
								matches = false
								break
							}
						}

						if matches {
							matching = append(matching, svc.Id)
						}
					}

					return types.NewStringList(types.DefaultTypeAdapter, matching)
				}),
			),
		),
	}
}

func (*serviceSupport) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// ServiceSupport creates a CEL library that provides service matching functionality.
// servicesByNamespace is a map from namespace to list of ServiceInfo.
func ServiceSupport(servicesByNamespace map[string][]ServiceInfo) cel.EnvOption {
	if servicesByNamespace == nil {
		servicesByNamespace = make(map[string][]ServiceInfo)
	}
	return cel.Lib(&serviceSupport{servicesByNamespace: servicesByNamespace})
}

// MatchesSelector checks if podLabels contain all key-value pairs from selector
func MatchesSelector(selector, podLabels map[string]string) bool {
	if len(selector) == 0 {
		return false // Services without selector don't select any pods
	}
	for k, v := range selector {
		if podLabels[k] != v {
			return false
		}
	}
	return true
}

func init() {
	// Register the type for error messages
	_ = fmt.Sprintf
}
