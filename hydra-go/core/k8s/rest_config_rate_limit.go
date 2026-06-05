package k8s

import (
	"k8s.io/client-go/rest"
)

// ApplyRESTConfigRateLimits configures client-go REST request throttling on cfg (typically right after
// genericclioptions.ConfigFlags.ToRESTConfig). When both qps and burst are zero, cfg is unchanged.
// Negative qps disables client-side rate limiting (see k8s.io/client-go/rest.Config). Positive qps sets
// the token bucket rate; burst zero leaves rest.Config.Burst at zero so RESTClientFor uses DefaultBurst.
func ApplyRESTConfigRateLimits(cfg *rest.Config, qps float32, burst int) {
	if qps == 0 && burst == 0 {
		return
	}
	if qps < 0 {
		cfg.QPS = qps
		return
	}
	cfg.QPS = qps
	if burst > 0 {
		cfg.Burst = burst
	}
}
