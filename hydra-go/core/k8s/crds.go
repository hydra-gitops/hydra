package k8s

import (
	"context"
	"sync"
	"time"

	herrors "hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

func ParseCRD(u unstructured.Unstructured) (*apiextv1.CustomResourceDefinition, error) {
	var crd apiextv1.CustomResourceDefinition
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &crd)
	if err != nil {
		return nil, err
	}
	return &crd, nil
}

var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

func WaitForCRDsEstablished(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	crdNames []string,
	timeout time.Duration,
) error {
	if len(crdNames) == 0 {
		return nil
	}

	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		first error
	)

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for _, name := range crdNames {
		wg.Add(1)
		go func(crdName string) {
			defer wg.Done()
			err := waitForSingleCRD(timeoutCtx, l, dynamicClient, crdName, timeout)
			if err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
			}
		}(name)
	}

	wg.Wait()
	return first
}

func waitForSingleCRD(ctx context.Context, l log.Logger, dynamicClient dynamic.Interface, crdName string, timeout time.Duration) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	l.Info(logIdK8s, "waiting for CRD {crd} to become Established...", log.String("crd", crdName))

	for {
		obj, err := dynamicClient.Resource(crdGVR).Get(ctx, crdName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return log.CreateError(herrors.ErrCrdEstablishTimeout,
					"CRD {crd} not found", log.String("crd", crdName), log.Err(err))
			}
		} else if isCRDEstablished(obj) {
			l.Info(logIdK8s, "CRD {crd} is Established", log.String("crd", crdName))
			return nil
		}

		select {
		case <-ctx.Done():
			return log.CreateError(herrors.ErrCrdEstablishTimeout,
				"aborted: CRDs did not become Established within {timeout}. Re-running the command is safe.",
				log.String("timeout", timeout.String()))
		case <-ticker.C:
		}
	}
}

func isCRDEstablished(obj *unstructured.Unstructured) bool {
	status, ok := obj.Object["status"].(map[string]interface{})
	if !ok {
		return false
	}
	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		return false
	}
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if ok && cond["type"] == "Established" && cond["status"] == "True" {
			return true
		}
	}
	return false
}
