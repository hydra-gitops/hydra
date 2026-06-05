package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

const (
	fieldManagerHydra           = "hydra"
	lastAppliedConfigAnnotation = "kubectl.kubernetes.io/last-applied-configuration"
)

// ApplyCallback is called for each successfully applied resource with the
// formatted resource name (e.g. "configmap.my-cm"), the Hydra resource id
// (e.g. "v1/ConfigMap/ns/my-cm"), and the action result (e.g. "configured",
// "created", "unchanged", "serverside-applied").
type ApplyCallback func(resource string, action string, id types.Id)

func Apply(
	ctx context.Context,
	l log.Logger,
	cc *ClusterClient,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	dryRun types.DryRun,
	serverSideApply bool,
	deleteBeforeApply sets.Set[types.Id],
	onApply ApplyCallback,
	useFooter bool,
) (err error) {
	dryRunPrefix := ""
	if dryRun {
		dryRunPrefix = "[dry-run] "
	}

	if onApply == nil {
		onApply = func(resource string, action string, id types.Id) {
			l.Info(logIdK8s, dryRunPrefix+"{id}: {resource}: {action}",
				log.String("id", string(id)),
				log.String("resource", resource),
				log.String("action", action))
		}
	}

	dryRunOpt := dryRunOption(dryRun)

	total := len(entities.Items)
	var phaseOp string
	var bar log.Progress
	var detailTask log.ProgressTask
	if useFooter && total > 0 {
		phaseOp = "apply"
		if dryRun {
			phaseOp = "dry-run apply"
		}
		if serverSideApply {
			phaseOp += " (ssa)"
		}
		FlushProgressLogBeforeFooter()
		bar, err = l.NewProgress(phaseOp, total)
		if err != nil {
			return err
		}
		detailTask = bar.NewTask("")
		defer func() {
			if bar == nil {
				return
			}
			_ = bar.Close()
			summary := ""
			if err != nil {
				summary = fmt.Sprintf("%s: aborted: %v", phaseOp, err)
			} else {
				summary = fmt.Sprintf("%s: completed %d resource(s)", phaseOp, total)
			}
			l.Info(logIdK8s, summary)
		}()
	}

	for i, e := range entities.Items {
		id, err := e.Id()
		if err != nil {
			return err
		}

		u, ok := e.Unstructured(key)
		if !ok {
			return log.CreateError(errors.ErrKubeCtlApplyFailed,
				"entity {entity} has no unstructured data", log.String("entity", string(id)))
		}

		gvk := u.GroupVersionKind()
		mapping, err := cc.RESTMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return log.CreateError(errors.ErrKubeCtlApplyFailed,
				"cannot resolve GVR for {entity}", log.String("entity", string(id)), log.Err(err))
		}

		rc := resourceClient(cc.Dynamic, mapping, u.GetNamespace())
		name := u.GetName()

		if deleteBeforeApply != nil && deleteBeforeApply.Has(id) {
			if err = deleteExistingIfPresent(ctx, rc, name, dryRunOpt); err != nil {
				return log.CreateError(errors.ErrKubeCtlApplyFailed,
					"failed to delete {entity} before replace", log.String("entity", string(id)), log.Err(err))
			}
		}

		var result string
		if serverSideApply {
			result, err = applySSA(ctx, rc, &u, name, dryRunOpt)
		} else {
			result, err = applyClientSide(ctx, rc, &u, name, dryRunOpt)
		}
		if err != nil {
			msg := "failed to apply {entity}"
			if deleteBeforeApply == nil || !deleteBeforeApply.Has(id) {
				msg += "; immutable field conflicts are recreated automatically when the API reports them; otherwise re-run with --replace"
			}
			return log.CreateError(errors.ErrKubeCtlApplyFailed,
				msg, log.String("entity", string(id)), log.Err(err))
		}

		if bar != nil && detailTask != nil {
			detailTask.SetDetail(TruncateFooterDetail(string(id)))
			bar.Advance(i+1, total)
		}

		onApply(formatResourceName(gvk, name), result, id)
	}

	l.DebugLog(logIdK8s, dryRunPrefix+"finished applying resources")
	return nil
}

func deleteExistingIfPresent(
	ctx context.Context,
	rc dynamic.ResourceInterface,
	name string,
	dryRun []string,
) error {
	_, err := rc.Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get before replace: %w", err)
	}
	err = rc.Delete(ctx, name, metav1.DeleteOptions{DryRun: dryRun})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete before replace: %w", err)
	}
	return nil
}

func applySSA(
	ctx context.Context,
	rc dynamic.ResourceInterface,
	obj *unstructured.Unstructured,
	name string,
	dryRun []string,
) (string, error) {
	data, err := json.Marshal(obj.Object)
	if err != nil {
		return "", fmt.Errorf("marshal object: %w", err)
	}

	force := true
	_, err = rc.Patch(ctx, name, k8stypes.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: fieldManagerHydra,
		Force:        &force,
		DryRun:       dryRun,
	})
	if err != nil {
		return "", err
	}

	return "serverside-applied", nil
}

func applyClientSide(
	ctx context.Context,
	rc dynamic.ResourceInterface,
	obj *unstructured.Unstructured,
	name string,
	dryRun []string,
) (string, error) {
	desired := obj.DeepCopy()

	modified, err := json.Marshal(desired.Object)
	if err != nil {
		return "", fmt.Errorf("marshal desired object: %w", err)
	}

	setLastAppliedAnnotation(desired, string(modified))

	current, err := rc.Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		_, cErr := rc.Create(ctx, desired, metav1.CreateOptions{DryRun: dryRun})
		if cErr != nil {
			return "", cErr
		}
		return "created", nil
	}
	if err != nil {
		return "", fmt.Errorf("get current object: %w", err)
	}

	original := []byte("{}")
	if ann := current.GetAnnotations(); ann != nil {
		if v, ok := ann[lastAppliedConfigAnnotation]; ok {
			original = []byte(v)
		}
	}

	currentJSON, err := json.Marshal(current.Object)
	if err != nil {
		return "", fmt.Errorf("marshal current object: %w", err)
	}

	modifiedWithAnnotation, err := json.Marshal(desired.Object)
	if err != nil {
		return "", fmt.Errorf("marshal modified object: %w", err)
	}

	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(original, modifiedWithAnnotation, currentJSON)
	if err != nil {
		return "", fmt.Errorf("create merge patch: %w", err)
	}

	if string(patch) == "{}" {
		return "unchanged", nil
	}

	_, err = rc.Patch(ctx, name, k8stypes.MergePatchType, patch, metav1.PatchOptions{DryRun: dryRun})
	if err != nil {
		return "", err
	}

	return "configured", nil
}

func setLastAppliedAnnotation(obj *unstructured.Unstructured, configJSON string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[lastAppliedConfigAnnotation] = configJSON
	obj.SetAnnotations(annotations)
}

func resourceClient(dynClient dynamic.Interface, mapping *meta.RESTMapping, namespace string) dynamic.ResourceInterface {
	if mapping.Scope.Name() == meta.RESTScopeNameRoot {
		return dynClient.Resource(mapping.Resource)
	}
	return dynClient.Resource(mapping.Resource).Namespace(namespace)
}

func formatResourceName(gvk schema.GroupVersionKind, name string) string {
	kindLower := strings.ToLower(gvk.Kind)
	if gvk.Group != "" {
		kindLower = kindLower + "." + gvk.Group
	}
	return fmt.Sprintf("%s/%s", kindLower, name)
}

func dryRunOption(dryRun types.DryRun) []string {
	if dryRun {
		return []string{metav1.DryRunAll}
	}
	return nil
}
