package k8s

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
)

type ClusterClient struct {
	Dynamic    dynamic.Interface
	RESTMapper meta.RESTMapper
}

func NewClusterClient(configFlags *genericclioptions.ConfigFlags) (*ClusterClient, error) {
	return NewClusterClientWithRESTOverrides(configFlags, 0, 0)
}

// NewClusterClientWithRESTOverrides builds a ClusterClient after applying optional REST QPS/burst
// overrides (see ApplyRESTConfigRateLimits). Pass qps 0 and burst 0 to keep client-go defaults from kubeconfig.
func NewClusterClientWithRESTOverrides(configFlags *genericclioptions.ConfigFlags, apiQPS float32, apiBurst int) (*ClusterClient, error) {
	restConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	ApplyRESTConfigRateLimits(restConfig, apiQPS, apiBurst)
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	mapper, err := configFlags.ToRESTMapper()
	if err != nil {
		return nil, err
	}
	return &ClusterClient{
		Dynamic:    dynamicClient,
		RESTMapper: mapper,
	}, nil
}
