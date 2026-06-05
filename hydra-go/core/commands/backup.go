package commands

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"hydra-gitops.org/hydra/hydra-go/base/colors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	"hydra-gitops.org/hydra/hydra-go/core/sops"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	hyaml "hydra-gitops.org/hydra/hydra-go/core/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	kyaml "sigs.k8s.io/yaml"
)

// isBackupSopsSecret returns true if the SopsSecret has the AnnotationHydraBackup annotation.
func isBackupSopsSecret(u unstructured.Unstructured) bool {
	return u.GetAnnotations()[hydra.AnnotationHydraBackup] == "true"
}

// BackupResult describes the outcome for a single secret during backup create or restore.
type BackupResult struct {
	SecretId string
	Status   BackupStatus
	Diff     string
}

type BackupStatus string

const (
	BackupStatusUpToDate       BackupStatus = "up-to-date"
	BackupStatusBackedUp       BackupStatus = "backed-up"
	BackupStatusRestored       BackupStatus = "restored"
	BackupStatusSkipped        BackupStatus = "skipped"
	BackupStatusAlreadyExists  BackupStatus = "already-exists"
	BackupStatusWouldOverwrite BackupStatus = "would-overwrite"
	BackupStatusForceRestored  BackupStatus = "force-restored"
	BackupListFound            BackupStatus = "list-found"
	BackupListUpToDate         BackupStatus = "list-up-to-date"
	BackupListChanged          BackupStatus = "list-changed"
	BackupListNotInCluster     BackupStatus = "list-not-in-cluster"
	BackupListNoBackup         BackupStatus = "list-no-backup"
)

type BackupCreateOptions struct {
	SecretPredicates            []types.CelPredicate
	SkipFoundDefinitionsInfoLog bool
}

type BackupRestoreOptions struct {
	SecretPredicates        []types.CelPredicate
	CreateMissingNamespaces bool
}

// clusterSecrets holds all v1/Secret resources fetched from the cluster in a
// single API call. Use get() for direct lookups and entities() for CEL evaluation.
type clusterSecrets struct {
	byId       map[string]unstructured.Unstructured
	entityList []entity.Entity
}

func (cs *clusterSecrets) get(namespace, name string) (*unstructured.Unstructured, bool) {
	id := fmt.Sprintf("v1/Secret/%s/%s", namespace, name)
	u, ok := cs.byId[id]
	if !ok {
		return nil, false
	}
	return &u, true
}

func (cs *clusterSecrets) entities() []entity.Entity {
	return cs.entityList
}

// listClusterSecrets fetches all v1/Secret resources across all namespaces in a
// single API call.
func listClusterSecrets(cluster *hydra.Cluster) (*clusterSecrets, error) {
	restConfig, err := RestConfigForHydra(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	secretList, err := dynamicClient.Resource(gvr).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster secrets: %w", err)
	}

	cs := &clusterSecrets{
		byId: make(map[string]unstructured.Unstructured, len(secretList.Items)),
	}

	v1Api := types.NewApiVersion(types.Group(""), types.Version("v1"))
	for _, u := range secretList.Items {
		id := fmt.Sprintf("v1/Secret/%s/%s", u.GetNamespace(), u.GetName())
		cs.byId[id] = u

		e, buildErr := entity.NewEntityBuilder().
			WithApiVersion(v1Api).
			WithKind(types.Kind("Secret")).
			WithResource(types.Resource("secrets")).
			WithNamespaced(true).
			WithNamespace(types.Namespace(u.GetNamespace())).
			WithName(types.Name(u.GetName())).
			WithUnstructured(types.KeyClusterEntity, u).
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		cs.entityList = append(cs.entityList, e)
	}

	return cs, nil
}

// filterSecretsByPredicate evaluates a compiled CEL predicate against
// pre-fetched cluster secret entities and returns matching ones.
func filterSecretsByPredicate(secrets *clusterSecrets, predicate cel.Predicate) ([]entity.Entity, error) {
	var matched []entity.Entity
	for _, e := range secrets.entities() {
		ok, err := predicate.EvalBool(e, types.MissingKeysError)
		if err != nil {
			return nil, err
		}
		if ok {
			matched = append(matched, e)
		}
	}
	return matched, nil
}

// compileSecretSelectionPredicate builds a predicate for optional CLI secret filters.
// rendered is the same selected-app template render as backup predicate collection when expressions
// may reference managedNamespaces() / templateEntities() / clusterEntities() / entities().
func compileSecretSelectionPredicate(predicates []types.CelPredicate, rendered entity.Entities) (cel.Predicate, error) {
	env, err := cel.NewEnvWithEntityInventory(rendered)
	if err != nil {
		return nil, err
	}

	return env.CompilePredicate(append(
		[]types.CelPredicate{
			types.KubernetesGvkV1Secret.CelPredicate(),
		},
		predicates...,
	)...)
}

func newSecretEntity(secretObj map[string]any, keys ...types.EntityKeyUnstructured) (entity.Entity, error) {
	u := unstructured.Unstructured{Object: deepCopyMap(secretObj)}
	gvk := types.NewGVKFromK8s(u.GroupVersionKind())

	b := entity.NewEntityBuilder().
		WithGVK(gvk).
		WithResource(types.Resource("secrets")).
		WithName(types.Name(u.GetName()))

	for _, key := range keys {
		b = b.WithUnstructured(key, u)
	}

	if namespace := u.GetNamespace(); namespace != "" {
		b = b.WithNamespace(types.Namespace(namespace)).
			WithNamespaced(types.NamespacedNo)
	}

	return b.Build()
}

func matchesSecretSelection(predicate cel.Predicate, secretObj map[string]any) (bool, error) {
	if predicate == nil {
		return true, nil
	}
	e, err := newSecretEntity(secretObj, types.KeyClusterEntity, types.KeyTemplateEntity)
	if err != nil {
		return false, err
	}
	return predicate.EvalBool(e, types.MissingKeysError)
}

// BackupCreate creates backups for all secrets matched by backup predicates of the given apps.
func BackupCreate(
	cluster *hydra.Cluster,
	appIds []types.AppId,
	networkMode types.HelmNetworkMode,
	color types.Color,
	dryRun types.DryRun,
) ([]BackupResult, error) {
	return BackupCreateWithOptions(cluster, appIds, networkMode, color, dryRun, BackupCreateOptions{})
}

// BackupCreateWithOptions creates backups for all secrets matched by backup predicates
// of the given apps and applies optional additional secret selection.
func BackupCreateWithOptions(
	cluster *hydra.Cluster,
	appIds []types.AppId,
	networkMode types.HelmNetworkMode,
	color types.Color,
	dryRun types.DryRun,
	options BackupCreateOptions,
) ([]BackupResult, error) {
	l := cluster.L()

	appIdSet := sets.New[types.AppId]()
	for _, id := range appIds {
		appIdSet.Insert(id)
	}

	var renderOptions []RenderClusterSelectedAppsOption
	if options.SkipFoundDefinitionsInfoLog {
		renderOptions = append(renderOptions, WithSkipFoundDefinitionsInfoLog())
	}

	rendered, err := RenderClusterSelectedApps(cluster, networkMode, "", appIdSet, types.KeyTemplateEntity, renderOptions...)
	if err != nil {
		return nil, err
	}
	groups, err := collectBackupGroupsWithRendered(cluster, appIds, networkMode, rendered)
	if err != nil {
		return nil, err
	}

	if len(groups) == 0 {
		l.Info(logIdCommands, "no backup predicates found for selected apps")
		return nil, nil
	}
	secrets, err := listClusterSecrets(cluster)
	if err != nil {
		return nil, err
	}

	secretSelection, err := compileSecretSelectionPredicate(options.SecretPredicates, rendered)
	if err != nil {
		return nil, err
	}

	var results []BackupResult

	for _, group := range groups {
		env, err := cel.NewEnvWithEntityInventory(rendered)
		if err != nil {
			return nil, err
		}

		for _, predicateCtx := range group.Predicates {
			predicate := strings.TrimSpace(predicateCtx.Predicate)
			if predicate == "" {
				return nil, fmt.Errorf("empty backup predicate for %s", predicateCtx.Summary())
			}
			programs, err := env.CompilePredicate("clusterEntity != null", types.KubernetesGvkV1Secret.CelPredicate(), types.CelPredicate(predicate))
			if err != nil {
				return nil, fmt.Errorf("backup predicate compile failed for %s expression=%q: %w", predicateCtx.Summary(), predicate, err)
			}

			matched, err := filterSecretsByPredicate(secrets, programs)
			if err != nil {
				return nil, err
			}

			for _, e := range matched {
				u, ok := e.Unstructured(types.KeyClusterEntity)
				if !ok {
					continue
				}

				selected, err := matchesSecretSelection(secretSelection, u.Object)
				if err != nil {
					return nil, err
				}
				if !selected {
					continue
				}

				name := u.GetName()
				namespace := u.GetNamespace()
				secretId := fmt.Sprintf("v1/Secret/%s/%s", namespace, name)

				if err := validateBackupCreateOwnership(group, u); err != nil {
					return nil, fmt.Errorf("backup failed for %s: %w", secretId, err)
				}

				backupBase := fmt.Sprintf("backup-%s-%s.sops.yaml", namespace, name)
				var backupDir, backupFile string
				if group.ChildAppName == "" {
					// Root app: Sops backups live next to Chart.yaml (Hydra also loads them from there).
					backupDir = group.RootAppPath
					backupFile = filepath.Join(backupDir, backupBase)
				} else {
					backupDir = filepath.Join(group.RootAppPath, "apps", group.ChildAppName)
					backupFile = filepath.Join(backupDir, backupBase)
				}

				result, err := backupSingleSecret(l, u, backupFile, backupDir, secretId, color, dryRun)
				if err != nil {
					return nil, fmt.Errorf("backup failed for %s: %w", secretId, err)
				}
				results = append(results, result)
			}
		}
	}

	return results, nil
}

type backupRestoreCandidate struct {
	backupFile string
	secretId   string
	secretObj  map[string]any
}

// BackupRestore restores secrets from backup SopsSecrets found in rendered manifests.
func BackupRestore(
	cluster *hydra.Cluster,
	appIds []types.AppId,
	networkMode types.HelmNetworkMode,
	kubernetesVersion types.KubernetesVersion,
	forceRestore bool,
	color types.Color,
	dryRun types.DryRun,
) ([]BackupResult, error) {
	return BackupRestoreWithOptions(cluster, appIds, networkMode, kubernetesVersion, forceRestore, color, dryRun, BackupRestoreOptions{})
}

// BackupRestoreWithOptions restores secrets from backup SopsSecrets found in
// manifests rendered for the selected app IDs and applies optional secret selection.
func BackupRestoreWithOptions(
	cluster *hydra.Cluster,
	appIds []types.AppId,
	networkMode types.HelmNetworkMode,
	kubernetesVersion types.KubernetesVersion,
	forceRestore bool,
	color types.Color,
	dryRun types.DryRun,
	options BackupRestoreOptions,
) ([]BackupResult, error) {
	l := cluster.L()

	backups, err := BackupSopsSecrets(cluster, appIds, networkMode, kubernetesVersion)
	if err != nil {
		return nil, err
	}

	if len(backups) == 0 {
		l.Info(logIdCommands, "no backup SopsSecrets found in rendered manifests")
		return nil, nil
	}

	candidates, skippedResults, err := collectBackupRestoreCandidates(backups, options.SecretPredicates)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 && len(skippedResults) == 0 {
		l.Info(logIdCommands, "no backup secrets matched the selected app IDs and secret filters")
		return nil, nil
	}

	if options.CreateMissingNamespaces {
		existingNamespaces, err := listClusterNamespaces(cluster)
		if err != nil {
			return nil, err
		}

		missingNamespaces := collectMissingBackupRestoreNamespaces(candidates, existingNamespaces)
		if err := ensureBackupRestoreNamespaces(cluster, missingNamespaces, dryRun); err != nil {
			return nil, err
		}
	}

	secrets, err := listClusterSecrets(cluster)
	if err != nil {
		return nil, err
	}

	results := make([]BackupResult, 0, len(skippedResults)+len(candidates))
	results = append(results, skippedResults...)
	for _, candidate := range candidates {
		result, err := restoreSingleBackup(l, cluster, secrets, candidate.backupFile, forceRestore, color, dryRun)
		if err != nil {
			return nil, fmt.Errorf("restore failed for %s: %w", candidate.secretId, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// BackupList lists backup SopsSecrets found in rendered manifests (no cluster comparison).
func BackupList(
	cluster *hydra.Cluster,
	appIds []types.AppId,
	networkMode types.HelmNetworkMode,
	kubernetesVersion types.KubernetesVersion,
) ([]BackupResult, error) {
	l := cluster.L()

	backups, err := BackupSopsSecrets(cluster, appIds, networkMode, kubernetesVersion)
	if err != nil {
		return nil, err
	}

	if len(backups) == 0 {
		l.Info(logIdCommands, "no backup SopsSecrets found in rendered manifests")
		return nil, nil
	}

	var results []BackupResult
	for _, b := range backups {
		results = append(results, BackupResult{SecretId: b.SecretId, Status: BackupListFound})
	}

	return results, nil
}

// BackupDiff compares backup state with the cluster for all secrets matched by
// backup predicates. Secrets that have a backup SopsSecret are decrypted and
// compared as v1/Secret. Secrets matched by predicates but without a backup
// are reported as no-backup.
func BackupDiff(
	cluster *hydra.Cluster,
	appIds []types.AppId,
	networkMode types.HelmNetworkMode,
	kubernetesVersion types.KubernetesVersion,
	color types.Color,
) ([]BackupResult, error) {
	l := cluster.L()

	secrets, err := listClusterSecrets(cluster)
	if err != nil {
		return nil, err
	}

	backups, err := BackupSopsSecrets(cluster, appIds, networkMode, kubernetesVersion)
	if err != nil {
		return nil, err
	}

	backupBySecretId := make(map[string]BackupSopsSecretInfo, len(backups))
	for _, b := range backups {
		backupBySecretId[b.SecretId] = b
	}

	predicateSecretIds, err := collectPredicateMatchedSecretIds(secrets, cluster, appIds, networkMode)
	if err != nil {
		return nil, err
	}

	allSecretIds := sets.New[string]()
	for _, b := range backups {
		allSecretIds.Insert(b.SecretId)
	}
	for _, id := range predicateSecretIds {
		allSecretIds.Insert(id)
	}

	if allSecretIds.Len() == 0 {
		l.Info(logIdCommands, "no backup secrets found")
		return nil, nil
	}

	sortedIds := sets.List(allSecretIds)

	var results []BackupResult
	for _, secretId := range sortedIds {
		b, hasBackup := backupBySecretId[secretId]
		if !hasBackup {
			results = append(results, BackupResult{SecretId: secretId, Status: BackupListNoBackup})
			continue
		}

		backupSecretObj, err := decryptBackupToSecret(b.AbsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt backup for %s: %w", secretId, err)
		}

		backupNormalized := normalizeSecretData(backupSecretObj)
		name := stringFromMap(backupSecretObj, "metadata", "name")
		namespace := stringFromMap(backupSecretObj, "metadata", "namespace")

		clusterSecret, found := secrets.get(namespace, name)
		if !found {
			results = append(results, BackupResult{SecretId: secretId, Status: BackupListNotInCluster})
			continue
		}

		clusterNormalized := normalizeSecretData(clusterSecret.Object)
		diff := backupDiff(backupNormalized, clusterNormalized, secretId, color)

		if diff == "" {
			results = append(results, BackupResult{SecretId: secretId, Status: BackupListUpToDate})
		} else {
			results = append(results, BackupResult{SecretId: secretId, Status: BackupListChanged, Diff: diff})
		}
	}

	return results, nil
}

// collectPredicateMatchedSecretIds evaluates backup predicates against
// pre-fetched cluster secrets and returns matching secret IDs.
func collectPredicateMatchedSecretIds(
	secrets *clusterSecrets,
	cluster *hydra.Cluster,
	appIds []types.AppId,
	networkMode types.HelmNetworkMode,
) ([]string, error) {
	appIdSet := sets.New[types.AppId]()
	for _, id := range appIds {
		appIdSet.Insert(id)
	}
	rendered, err := RenderClusterSelectedApps(cluster, networkMode, "", appIdSet, types.KeyTemplateEntity)
	if err != nil {
		return nil, err
	}
	groups, err := collectBackupGroupsWithRendered(cluster, appIds, networkMode, rendered)
	if err != nil {
		return nil, err
	}

	var ids []string
	seen := sets.New[string]()

	for _, group := range groups {
		env, err := cel.NewEnvWithEntityInventory(rendered)
		if err != nil {
			return nil, err
		}

		for _, predicateCtx := range group.Predicates {
			predicate := strings.TrimSpace(predicateCtx.Predicate)
			if predicate == "" {
				return nil, fmt.Errorf("empty backup predicate for %s", predicateCtx.Summary())
			}
			programs, err := env.CompilePredicate("clusterEntity != null", types.KubernetesGvkV1Secret.CelPredicate(), types.CelPredicate(predicate))
			if err != nil {
				return nil, fmt.Errorf("backup predicate compile failed for %s expression=%q: %w", predicateCtx.Summary(), predicate, err)
			}

			matched, err := filterSecretsByPredicate(secrets, programs)
			if err != nil {
				return nil, err
			}

			for _, e := range matched {
				u, ok := e.Unstructured(types.KeyClusterEntity)
				if !ok {
					continue
				}
				secretId := fmt.Sprintf("v1/Secret/%s/%s", u.GetNamespace(), u.GetName())
				if !seen.Has(secretId) {
					seen.Insert(secretId)
					ids = append(ids, secretId)
				}
			}
		}
	}

	return ids, nil
}

func collectBackupRestoreCandidates(
	backups []BackupSopsSecretInfo,
	secretPredicates []types.CelPredicate,
) ([]backupRestoreCandidate, []BackupResult, error) {
	secretSelection, err := compileSecretSelectionPredicate(secretPredicates, entity.Entities{})
	if err != nil {
		return nil, nil, err
	}

	result := make([]backupRestoreCandidate, 0, len(backups))
	skipped := []BackupResult{}
	for _, backup := range backups {
		secretObj, err := decryptBackupToSecret(backup.AbsPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decrypt backup for %s: %w", backup.SecretId, err)
		}

		selected, err := matchesSecretSelection(secretSelection, secretObj)
		if err != nil {
			return nil, nil, err
		}
		if !selected {
			continue
		}

		namespace := types.Namespace(stringFromMap(secretObj, "metadata", "namespace"))
		name := stringFromMap(secretObj, "metadata", "name")
		secretId := fmt.Sprintf("v1/Secret/%s/%s", namespace, name)

		if namespace != backup.AppNamespace {
			l := log.Default()
			l.Warn(logIdCommands, "  {id}: skipped, target namespace does not belong to app {appId}",
				log.String("id", secretId),
				log.String("appId", string(backup.AppId)))
			skipped = append(skipped, BackupResult{
				SecretId: secretId,
				Status:   BackupStatusSkipped,
			})
			continue
		}

		result = append(result, backupRestoreCandidate{
			backupFile: backup.AbsPath,
			secretId:   secretId,
			secretObj:  secretObj,
		})
	}

	return result, skipped, nil
}

func listClusterNamespaces(cluster *hydra.Cluster) (sets.Set[types.Namespace], error) {
	restConfig, err := RestConfigForHydra(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	namespaceList, err := dynamicClient.Resource(gvr).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster namespaces: %w", err)
	}

	result := sets.New[types.Namespace]()
	for _, item := range namespaceList.Items {
		if item.GetName() == "" {
			continue
		}
		result.Insert(types.Namespace(item.GetName()))
	}

	return result, nil
}

func collectMissingBackupRestoreNamespaces(
	candidates []backupRestoreCandidate,
	existingNamespaces sets.Set[types.Namespace],
) sets.Set[types.Namespace] {
	missing := sets.New[types.Namespace]()
	for _, candidate := range candidates {
		namespace := types.Namespace(stringFromMap(candidate.secretObj, "metadata", "namespace"))
		if namespace == "" || existingNamespaces.Has(namespace) {
			continue
		}
		missing.Insert(namespace)
	}
	return missing
}

func ensureBackupRestoreNamespaces(cluster *hydra.Cluster, namespaces sets.Set[types.Namespace], dryRun types.DryRun) error {
	if namespaces.Len() == 0 {
		return nil
	}

	configFlags, err := hydra.HydraClusterAccess(cluster)
	if err != nil {
		return fmt.Errorf("failed to get cluster access: %w", err)
	}

	lim := cluster.RESTClientLimits
	cc, err := k8s.NewClusterClientWithRESTOverrides(configFlags, lim.QPS, lim.Burst)
	if err != nil {
		return fmt.Errorf("failed to create cluster client: %w", err)
	}

	namespaceEntities, err := CreateNamespaceEntities(namespaces, types.KeyTemplateEntity)
	if err != nil {
		return fmt.Errorf("failed to build namespace entities: %w", err)
	}

	return k8s.Apply(context.Background(), cluster.L(), cc, namespaceEntities, types.KeyTemplateEntity, dryRun, false, nil, nil, false)
}

func backupSingleSecret(
	l log.Logger,
	clusterSecret unstructured.Unstructured,
	backupFile string,
	backupDir string,
	secretId string,
	color types.Color,
	dryRun types.DryRun,
) (BackupResult, error) {
	clusterNormalized := normalizeSecretData(clusterSecret.Object)

	if _, err := os.Stat(backupFile); err == nil {
		backupSecretObj, err := decryptBackupToSecret(backupFile)
		if err != nil {
			return BackupResult{}, err
		}

		backupNormalized := normalizeSecretData(backupSecretObj)

		diff := backupDiff(backupNormalized, clusterNormalized, secretId, color)
		if diff == "" {
			l.Info(logIdCommands, "  {id}: up-to-date", log.String("id", secretId))
			return BackupResult{SecretId: secretId, Status: BackupStatusUpToDate}, nil
		}

		l.Info(logIdCommands, "  {id}: content changed, updating backup", log.String("id", secretId))
		fmt.Print(diff)
	} else {
		l.Info(logIdCommands, "  {id}: new backup", log.String("id", secretId))
	}

	if dryRun {
		l.Info(logIdCommands, "  {id}: dry-run, skipping write", log.String("id", secretId))
		return BackupResult{SecretId: secretId, Status: BackupStatusBackedUp}, nil
	}

	sopsSecretYaml, err := convertSecretToSopsSecretYaml(clusterSecret)
	if err != nil {
		return BackupResult{}, err
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return BackupResult{}, fmt.Errorf("failed to create backup directory %s: %w", backupDir, err)
	}

	if err := sops.EncryptSopsFile(sopsSecretYaml, backupFile); err != nil {
		return BackupResult{}, fmt.Errorf("failed to encrypt backup file %s: %w", backupFile, err)
	}

	return BackupResult{SecretId: secretId, Status: BackupStatusBackedUp}, nil
}

func validateBackupCreateOwnership(group backupGroup, clusterSecret unstructured.Unstructured) error {
	namespace := types.Namespace(clusterSecret.GetNamespace())
	if namespace == group.AppNamespace {
		return nil
	}

	return fmt.Errorf("backup secret namespace %q does not belong to app %q (app namespace %q)",
		namespace, group.AppId, group.AppNamespace)
}

func restoreSingleBackup(
	l log.Logger,
	cluster *hydra.Cluster,
	secrets *clusterSecrets,
	backupFile string,
	forceRestore bool,
	color types.Color,
	dryRun types.DryRun,
) (BackupResult, error) {
	backupSecretObj, err := decryptBackupToSecret(backupFile)
	if err != nil {
		return BackupResult{}, err
	}

	backupNormalized := normalizeSecretData(backupSecretObj)
	name := stringFromMap(backupSecretObj, "metadata", "name")
	namespace := stringFromMap(backupSecretObj, "metadata", "namespace")
	secretId := fmt.Sprintf("v1/Secret/%s/%s", namespace, name)

	clusterSecret, found := secrets.get(namespace, name)
	if !found {
		l.Info(logIdCommands, "  {id}: not found in cluster, will restore", log.String("id", secretId))

		if dryRun {
			l.Info(logIdCommands, "  {id}: dry-run, skipping restore", log.String("id", secretId))
			return BackupResult{SecretId: secretId, Status: BackupStatusRestored}, nil
		}

		if err := restoreSecretToCluster(cluster, backupSecretObj, dryRun); err != nil {
			return BackupResult{}, err
		}
		return BackupResult{SecretId: secretId, Status: BackupStatusRestored}, nil
	}

	clusterNormalized := normalizeSecretData(clusterSecret.Object)
	diff := backupDiff(backupNormalized, clusterNormalized, secretId, color)

	if diff == "" {
		return BackupResult{SecretId: secretId, Status: BackupStatusUpToDate}, nil
	}

	if !forceRestore {
		l.Warn(logIdCommands, "  {id}: would overwrite existing secret (use --force-backup-restore to override)", log.String("id", secretId))
		fmt.Print(diff)
		return BackupResult{SecretId: secretId, Status: BackupStatusWouldOverwrite, Diff: diff}, nil
	}

	l.Warn(logIdCommands, "  {id}: force-restoring (overwriting existing secret)", log.String("id", secretId))
	fmt.Print(diff)

	if dryRun {
		l.Info(logIdCommands, "  {id}: dry-run, skipping force-restore", log.String("id", secretId))
		return BackupResult{SecretId: secretId, Status: BackupStatusForceRestored, Diff: diff}, nil
	}

	if err := restoreSecretToCluster(cluster, backupSecretObj, dryRun); err != nil {
		return BackupResult{}, err
	}
	return BackupResult{SecretId: secretId, Status: BackupStatusForceRestored, Diff: diff}, nil
}

// decryptBackupToSecret decrypts a SOPS-encrypted SopsSecret backup file and
// extracts the first secret template as a normalized v1/Secret object map.
func decryptBackupToSecret(backupFile string) (map[string]any, error) {
	decryptedYaml, err := sops.DecryptSopsFile(backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt backup file %s: %w", backupFile, err)
	}

	decrypted, err := hyaml.YamlToUnstructured(decryptedYaml)
	if err != nil {
		return nil, fmt.Errorf("failed to parse decrypted backup %s: %w", backupFile, err)
	}

	namespace := decrypted.GetNamespace()
	templates, err := extractSecretTemplates(decrypted.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to extract secret templates from %s: %w", backupFile, err)
	}

	if len(templates) == 0 {
		return nil, fmt.Errorf("no secret templates found in %s", backupFile)
	}

	secretU := buildSecretUnstructured(templates[0], namespace)
	return secretU.Object, nil
}

// restoreSecretToCluster applies a v1/Secret directly to the cluster via the
// Kubernetes API. The backup file is never modified.
func restoreSecretToCluster(cluster *hydra.Cluster, secretObj map[string]any, dryRun types.DryRun) error {
	u := unstructured.Unstructured{Object: deepCopyMap(secretObj)}

	if md, ok := u.Object["metadata"].(map[string]any); ok {
		delete(md, "creationTimestamp")
		delete(md, "resourceVersion")
		delete(md, "uid")
		delete(md, "managedFields")
	}

	configFlags, err := hydra.HydraClusterAccess(cluster)
	if err != nil {
		return fmt.Errorf("failed to get cluster access: %w", err)
	}
	lim := cluster.RESTClientLimits
	cc, err := k8s.NewClusterClientWithRESTOverrides(configFlags, lim.QPS, lim.Burst)
	if err != nil {
		return fmt.Errorf("failed to create cluster client: %w", err)
	}

	secretEntity, err := newSecretEntity(secretObj, types.KeyTemplateEntity)
	if err != nil {
		return fmt.Errorf("failed to create secret entity: %w", err)
	}

	entities, err := entity.NewEntities([]entity.Entity{secretEntity})
	if err != nil {
		return fmt.Errorf("failed to create entity list: %w", err)
	}

	return k8s.Apply(context.Background(), cluster.L(), cc, entities, types.KeyTemplateEntity, dryRun, false, nil, nil, false)
}

// convertSecretToSopsSecretYaml converts a v1/Secret into a SopsSecret CRD YAML string.
func convertSecretToSopsSecretYaml(secret unstructured.Unstructured) (types.YamlString, error) {
	name := secret.GetName()
	namespace := secret.GetNamespace()
	secretType, _, _ := unstructured.NestedString(secret.Object, "type")

	template := map[string]any{
		"name": name,
	}

	if secretType != "" {
		template["type"] = secretType
	}

	if data, ok := secret.Object["data"].(map[string]any); ok && len(data) > 0 {
		template["data"] = data
	}

	if sd, ok := secret.Object["stringData"].(map[string]any); ok && len(sd) > 0 {
		template["stringData"] = sd
	}

	labels := secret.GetLabels()
	if len(labels) > 0 {
		filtered := filterBackupLabels(labels)
		if len(filtered) > 0 {
			template["labels"] = toAnyMap(filtered)
		}
	}

	annotations := secret.GetAnnotations()
	if len(annotations) > 0 {
		filtered := filterBackupAnnotations(annotations)
		if len(filtered) > 0 {
			template["annotations"] = toAnyMap(filtered)
		}
	}

	sopsSecret := map[string]any{
		"apiVersion": "isindir.github.com/v1alpha3",
		"kind":       "SopsSecret",
		"metadata": map[string]any{
			"name":      name + "-backup",
			"namespace": namespace,
			"annotations": map[string]any{
				hydra.AnnotationHydraBackup: "true",
			},
		},
		"spec": map[string]any{
			"suspend":         true,
			"secretTemplates": []any{template},
		},
	}

	yamlBytes, err := kyaml.Marshal(sopsSecret)
	if err != nil {
		return "", fmt.Errorf("failed to serialize SopsSecret: %w", err)
	}

	return types.YamlString(yamlBytes), nil
}

// normalizeSecretData converts stringData entries to base64-encoded data entries
// and removes the stringData field. If a key exists in both data and stringData,
// stringData wins (matching Kubernetes behavior).
func normalizeSecretData(obj map[string]any) map[string]any {
	normalized := make(map[string]any)
	for k, v := range obj {
		if k == "stringData" {
			continue
		}
		normalized[k] = v
	}

	data := make(map[string]string)

	if d, ok := obj["data"].(map[string]any); ok {
		for k, v := range d {
			if s, ok := v.(string); ok {
				data[k] = s
			}
		}
	}

	if sd, ok := obj["stringData"].(map[string]any); ok {
		for k, v := range sd {
			if s, ok := v.(string); ok {
				data[k] = base64.StdEncoding.EncodeToString([]byte(s))
			}
		}
	}

	if len(data) > 0 {
		anyData := make(map[string]any, len(data))
		for k, v := range data {
			anyData[k] = v
		}
		normalized["data"] = anyData
	} else {
		delete(normalized, "data")
	}

	normalizeSecretMetadataAnnotations(normalized)

	return normalized
}

func normalizeSecretMetadataAnnotations(obj map[string]any) {
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok || len(metadata) == 0 {
		return
	}

	annotations, ok := metadata["annotations"].(map[string]any)
	if !ok || len(annotations) == 0 {
		return
	}

	stringAnnotations := make(map[string]string, len(annotations))
	for k, v := range annotations {
		s, ok := v.(string)
		if !ok {
			continue
		}
		stringAnnotations[k] = s
	}

	filtered := filterBackupAnnotations(stringAnnotations)
	normalizedMetadata := deepCopyMap(metadata)
	if len(filtered) == 0 {
		delete(normalizedMetadata, "annotations")
	} else {
		normalizedMetadata["annotations"] = toAnyMap(filtered)
	}

	obj["metadata"] = normalizedMetadata
}

// secretHashedYaml produces a YAML representation of a normalized secret object
// where all data values are replaced with their truncated SHA256 hashes.
// Metadata, type, etc. are preserved in cleartext.
func secretHashedYaml(obj map[string]any) string {
	hashed := deepCopyMap(obj)

	if data, ok := hashed["data"].(map[string]any); ok {
		for k, v := range data {
			if s, ok := v.(string); ok {
				h := sha256.Sum256([]byte(s))
				data[k] = "sha256:" + hex.EncodeToString(h[:])[:12]
			}
		}
	}

	// Remove server-managed metadata fields for cleaner diffs
	if metadata, ok := hashed["metadata"].(map[string]any); ok {
		delete(metadata, "creationTimestamp")
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "managedFields")
	}

	yamlBytes, _ := kyaml.Marshal(hashed)
	return string(yamlBytes)
}

// backupDiff produces a colored unified diff between two normalized+hashed secrets.
// Returns empty string if secrets are identical.
func backupDiff(backupObj, clusterObj map[string]any, secretId string, color types.Color) string {
	oldYaml := secretHashedYaml(backupObj)
	newYaml := secretHashedYaml(clusterObj)

	if oldYaml == newYaml {
		return ""
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldYaml),
		B:        difflib.SplitLines(newYaml),
		FromFile: "backup/" + secretId,
		ToFile:   "cluster/" + secretId,
		Context:  3,
	}

	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil || text == "" {
		return ""
	}

	if color {
		return colors.ColorDiff(text)
	}
	return text
}

// PrintBackupResults logs the results of a backup operation to the console.
func PrintBackupResults(l log.Logger, results []BackupResult, color types.Color) {
	PrintBackupResultsWithSkippedHint(l, results, color, "")
}

// PrintBackupResultsWithSkippedHint logs the results of a backup operation to
// the console and appends a custom hint when skipped restore results occurred.
func PrintBackupResultsWithSkippedHint(l log.Logger, results []BackupResult, color types.Color, skippedHint string) {
	if len(results) == 0 {
		l.Info(logIdCommands, "no secrets found to process")
		return
	}

	var colorOk, colorWarn, colorKey, colorReset string
	if color {
		colorOk = colors.Green.String()
		colorWarn = colors.LightYellow.String()
		colorKey = colors.LightMagenta.String()
		colorReset = colors.Reset.String()
	}

	fmt.Println()
	fmt.Println("Backup overview:")

	sorted := slices.Clone(results)
	slices.SortFunc(sorted, func(a, b BackupResult) int {
		return cmp.Compare(a.SecretId, b.SecretId)
	})
	for _, r := range sorted {
		switch r.Status {
		case BackupStatusUpToDate:
			fmt.Printf("%s  ✓ %s%s%s: up-to-date%s\n", colorOk, colorKey, r.SecretId, colorOk, colorReset)
		case BackupStatusBackedUp:
			fmt.Printf("%s  ✓ %s%s%s: backed up%s\n", colorOk, colorKey, r.SecretId, colorOk, colorReset)
		case BackupStatusRestored:
			fmt.Printf("%s  ✓ %s%s%s: restored%s\n", colorOk, colorKey, r.SecretId, colorOk, colorReset)
		case BackupStatusSkipped:
			fmt.Printf("%s  ⚠ %s%s%s: skipped%s\n", colorWarn, colorKey, r.SecretId, colorWarn, colorReset)
		case BackupStatusAlreadyExists:
			fmt.Printf("%s  ✓ %s%s%s: already exists%s\n", colorOk, colorKey, r.SecretId, colorOk, colorReset)
		case BackupStatusWouldOverwrite:
			fmt.Printf("%s  ⚠ %s%s%s: would overwrite (use --force-backup-restore)%s\n", colorWarn, colorKey, r.SecretId, colorWarn, colorReset)
		case BackupStatusForceRestored:
			fmt.Printf("%s  ✓ %s%s%s: force-restored%s\n", colorWarn, colorKey, r.SecretId, colorWarn, colorReset)
		case BackupListFound:
			fmt.Printf("%s  ✓ %s%s%s%s\n", colorOk, colorKey, r.SecretId, colorOk, colorReset)
		case BackupListUpToDate:
			fmt.Printf("%s  ✓ %s%s%s: backup up-to-date%s\n", colorOk, colorKey, r.SecretId, colorOk, colorReset)
		case BackupListChanged:
			fmt.Printf("%s  ⚠ %s%s%s: backup differs from cluster%s\n", colorWarn, colorKey, r.SecretId, colorWarn, colorReset)
			if r.Diff != "" {
				fmt.Print(r.Diff)
			}
		case BackupListNotInCluster:
			fmt.Printf("%s  ⚠ %s%s%s: not found in cluster%s\n", colorWarn, colorKey, r.SecretId, colorWarn, colorReset)
		case BackupListNoBackup:
			fmt.Printf("%s  ⚠ %s%s%s: no backup found%s\n", colorWarn, colorKey, r.SecretId, colorWarn, colorReset)
		}
	}

	if HasSkipped(results) && skippedHint != "" {
		fmt.Println()
		fmt.Println(skippedHint)
	}
}

// HasConflicts returns true if any result has a would-overwrite status.
func HasConflicts(results []BackupResult) bool {
	for _, r := range results {
		if r.Status == BackupStatusWouldOverwrite {
			return true
		}
	}
	return false
}

// HasSkipped returns true if any result was skipped during restore.
func HasSkipped(results []BackupResult) bool {
	for _, r := range results {
		if r.Status == BackupStatusSkipped {
			return true
		}
	}
	return false
}

// BackupSopsSecretInfo holds info about a backup SopsSecret found in manifests
// rendered for the selected app IDs.
type BackupSopsSecretInfo struct {
	SecretId     string
	AbsPath      string
	AppId        types.AppId
	AppNamespace types.Namespace
}

// BackupSopsSecrets renders the selected apps and finds SopsSecret entities
// with the AnnotationHydraBackup annotation.
func BackupSopsSecrets(
	cluster *hydra.Cluster,
	appIds []types.AppId,
	networkMode types.HelmNetworkMode,
	kubernetesVersion types.KubernetesVersion,
) ([]BackupSopsSecretInfo, error) {
	appIdSet := sets.New[types.AppId]()
	for _, id := range appIds {
		appIdSet.Insert(id)
	}

	rendered, err := RenderClusterSelectedApps(cluster, networkMode, kubernetesVersion, appIdSet, types.KeyTemplateEntity, WithSkipFoundDefinitionsInfoLog())
	if err != nil {
		return nil, fmt.Errorf("failed to render apps for backup discovery: %w", err)
	}

	var result []BackupSopsSecretInfo
	for _, e := range rendered.Items {
		kind, err := e.Kind()
		if err != nil || string(kind) != sopsSecretKind {
			continue
		}

		u, ok := e.Unstructured(types.KeyTemplateEntity)
		if !ok {
			continue
		}

		annotations := u.GetAnnotations()
		if annotations[hydra.AnnotationHydraBackup] != "true" {
			continue
		}

		absPath, _ := e.AbsPath()
		if absPath == "" {
			continue
		}

		appIds, appIdErr := e.AppIds()
		if appIdErr != nil || len(appIds) == 0 {
			continue
		}

		appNamespace, appNamespaceErr := e.AppNamespace()
		if appNamespaceErr != nil || appNamespace == "" {
			continue
		}

		templates, tErr := extractSecretTemplates(u.Object)
		if tErr != nil || len(templates) == 0 {
			continue
		}

		namespace := u.GetNamespace()
		tmplName := templates[0].Name
		secretId := fmt.Sprintf("v1/Secret/%s/%s", namespace, tmplName)

		result = append(result, BackupSopsSecretInfo{
			SecretId:     secretId,
			AbsPath:      string(absPath),
			AppId:        appIds[0],
			AppNamespace: types.Namespace(appNamespace),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].SecretId < result[j].SecretId
	})

	return result, nil
}

// BackupRestoreCandidateInfo identifies one v1/Secret that integrated backup restore would apply
// for the selected apps, using the same discovery and filtering as BackupRestoreWithOptions with
// default options (empty secret selection predicates).
type BackupRestoreCandidateInfo struct {
	SecretId   string
	BackupFile string
}

// ListBackupRestoreCandidates returns secrets that would be restored by BackupRestoreWithOptions
// with default options. It returns nil when there are no backup SopsSecrets or no restore candidates.
func ListBackupRestoreCandidates(
	cluster *hydra.Cluster,
	appIds []types.AppId,
	networkMode types.HelmNetworkMode,
	kubernetesVersion types.KubernetesVersion,
) ([]BackupRestoreCandidateInfo, error) {
	backups, err := BackupSopsSecrets(cluster, appIds, networkMode, kubernetesVersion)
	if err != nil {
		return nil, err
	}
	if len(backups) == 0 {
		return nil, nil
	}
	candidates, _, err := collectBackupRestoreCandidates(backups, nil)
	if err != nil {
		return nil, err
	}
	out := make([]BackupRestoreCandidateInfo, len(candidates))
	for i, c := range candidates {
		out[i] = BackupRestoreCandidateInfo{
			SecretId:   c.secretId,
			BackupFile: c.backupFile,
		}
	}
	return out, nil
}

// BackupRestoreCandidateSecretIDs returns the set of v1/Secret ids that integrated backup restore
// would apply for the selected apps (same discovery as [ListBackupRestoreCandidates]). The set is
// nil when there are no candidates (no error).
func BackupRestoreCandidateSecretIDs(
	cluster *hydra.Cluster,
	appIds []types.AppId,
	networkMode types.HelmNetworkMode,
	kubernetesVersion types.KubernetesVersion,
) (sets.Set[types.Id], error) {
	candidates, err := ListBackupRestoreCandidates(cluster, appIds, networkMode, kubernetesVersion)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	out := sets.New[types.Id]()
	for _, c := range candidates {
		out.Insert(types.Id(c.SecretId))
	}
	return out, nil
}

// BootstrapBackupApplyConflict is one v1/Secret id that would be written both by backup restore
// and by the bootstrap apply set (used for hydra gitops apply --bootstrap validation).
type BootstrapBackupApplyConflict struct {
	SecretID        string
	BackupFile      string
	ApplySourceDesc string
}

func describeApplySecretSourceForBootstrapConflict(e entity.Entity) string {
	tp, err := e.TemplatePath()
	if err == nil && tp != "" {
		appIds, _ := e.AppIds()
		if len(appIds) > 0 {
			return fmt.Sprintf("template manifest %s (app %s)", tp, appIds[0])
		}
		return fmt.Sprintf("template manifest %s", tp)
	}
	appIds, _ := e.AppIds()
	if len(appIds) > 0 {
		return fmt.Sprintf("bootstrap-derived Secret from SopsSecret conversion (app %s)", appIds[0])
	}
	return "bootstrap-derived Secret from SopsSecret conversion"
}

// FindBootstrapBackupApplyConflicts returns v1/Secret ids that appear both as backup restore
// targets and as Secrets in the non-CRD apply entity set (templates and bootstrap-derived Secrets).
func FindBootstrapBackupApplyConflicts(
	candidates []BackupRestoreCandidateInfo,
	nonCrds entity.Entities,
) ([]BootstrapBackupApplyConflict, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	byID := make(map[string]BackupRestoreCandidateInfo, len(candidates))
	for _, c := range candidates {
		byID[c.SecretId] = c
	}
	var out []BootstrapBackupApplyConflict
	for _, e := range nonCrds.Items {
		gvk, err := e.GVKString()
		if err != nil {
			return nil, err
		}
		if gvk != types.KubernetesGvkV1Secret {
			continue
		}
		id, err := e.Id()
		if err != nil {
			return nil, err
		}
		b, ok := byID[string(id)]
		if !ok {
			continue
		}
		out = append(out, BootstrapBackupApplyConflict{
			SecretID:        string(id),
			BackupFile:      b.BackupFile,
			ApplySourceDesc: describeApplySecretSourceForBootstrapConflict(e),
		})
	}
	slices.SortFunc(out, func(a, b BootstrapBackupApplyConflict) int {
		return cmp.Compare(a.SecretID, b.SecretID)
	})
	return out, nil
}

// backupGroup holds the information needed to backup secrets for one root or child app.
// ChildAppName is empty for a root app (in-cluster.<root>); non-empty for a child app
// (in-cluster.<root>.<child>).
type backupGroup struct {
	AppId        types.AppId
	AppNamespace types.Namespace
	RootAppPath  string
	ChildAppName string
	Predicates   []hydra.HydraPredicateContext
}

func collectBackupGroupsWithRendered(
	cluster *hydra.Cluster,
	appIds []types.AppId,
	networkMode types.HelmNetworkMode,
	rendered entity.Entities,
) ([]backupGroup, error) {
	appIdSet := make(map[types.AppId]bool)
	for _, id := range appIds {
		appIdSet[id] = true
	}
	selectedAppIds := sets.New[types.AppId]()
	for _, id := range appIds {
		selectedAppIds.Insert(id)
	}
	contextsByApp := map[types.AppId][]hydra.HydraPredicateContext{}
	if rendered.Len() > 0 {
		predicateContexts, err := hydra.HydraAppBackupPredicateContexts(cluster, selectedAppIds, networkMode, rendered)
		if err != nil {
			return nil, err
		}
		for _, ctx := range predicateContexts {
			contextsByApp[ctx.AppId] = append(contextsByApp[ctx.AppId], ctx)
		}
	}

	var groups []backupGroup

	for _, appId := range appIds {
		app, err := cluster.WithApp(appId)
		if err != nil {
			return nil, err
		}

		appNamespace, err := app.Namespace(networkMode)
		if err != nil {
			return nil, err
		}

		predicates := contextsByApp[appId]
		if rendered.Len() == 0 {
			hv, err := hydra.HydraValues(app, networkMode)
			if err != nil {
				return nil, err
			}
			if hv == nil {
				continue
			}
			for groupName, group := range hv.Refs {
				if !group.IsEnabled() || !group.HasTag("backup") {
					continue
				}
				for parserIndex, rp := range group.RefParsers {
					pred, err := rp.MatchPredicate()
					if err != nil {
						return nil, fmt.Errorf("refs.%s.ref-parsers[%d]: %w", groupName, parserIndex, err)
					}
					if p := strings.TrimSpace(string(pred)); p != "" {
						predicates = append(predicates, hydra.HydraPredicateContext{
							AppId:       appId,
							Tag:         "backup",
							GroupName:   groupName,
							ParserIndex: parserIndex,
							BlockPath:   fmt.Sprintf("global.hydra.refs.%s.ref-parsers[%d]", groupName, parserIndex),
							Predicate:   p,
							Sources:     []string{"effective merged hydra values"},
						})
					}
				}
			}
		}

		if len(predicates) == 0 {
			continue
		}

		rootApp := app.AsRootApp()
		if rootApp == nil {
			childApp := app.AsChildApp()
			if childApp != nil {
				rootApp = childApp.AsRootApp()
			}
		}
		if rootApp == nil {
			return nil, fmt.Errorf("could not resolve root app for %s", appId)
		}

		childAppName := ""
		cn, err := appId.ChildAppName()
		if err != nil {
			return nil, err
		}
		if cn != nil {
			childAppName = string(*cn)
		}

		groups = append(groups, backupGroup{
			AppId:        appId,
			AppNamespace: types.Namespace(appNamespace),
			RootAppPath:  rootApp.RootAppPath().Path(),
			ChildAppName: childAppName,
			Predicates:   predicates,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		return string(groups[i].AppId) < string(groups[j].AppId)
	})

	return groups, nil
}

func deepCopyMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			result[k] = deepCopyMap(val)
		case []any:
			result[k] = deepCopySlice(val)
		default:
			result[k] = v
		}
	}
	return result
}

func deepCopySlice(s []any) []any {
	result := make([]any, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case map[string]any:
			result[i] = deepCopyMap(val)
		case []any:
			result[i] = deepCopySlice(val)
		default:
			result[i] = v
		}
	}
	return result
}

func toAnyMap(m map[string]string) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func stringFromMap(obj map[string]any, keys ...string) string {
	current := obj
	for i, key := range keys {
		if i == len(keys)-1 {
			if s, ok := current[key].(string); ok {
				return s
			}
			return ""
		}
		if next, ok := current[key].(map[string]any); ok {
			current = next
		} else {
			return ""
		}
	}
	return ""
}

// filterBackupLabels removes Kubernetes-managed labels that should not be backed up.
// The app.kubernetes.io/ prefix is preserved because it contains standard application
// labels (e.g. managed-by, name, instance) that are set by tools like cert-manager.
func filterBackupLabels(labels map[string]string) map[string]string {
	filtered := make(map[string]string)
	for k, v := range labels {
		if strings.HasPrefix(k, "app.kubernetes.io/") {
			filtered[k] = v
			continue
		}
		if strings.Contains(k, "kubernetes.io/") || strings.Contains(k, "helm.sh/") {
			continue
		}
		filtered[k] = v
	}
	return filtered
}

// filterBackupAnnotations removes Kubernetes-managed annotations that should not be backed up.
func filterBackupAnnotations(annotations map[string]string) map[string]string {
	filtered := make(map[string]string)
	for k, v := range annotations {
		if k == sopsSecretManagedAnnotationKey {
			continue
		}
		if strings.HasPrefix(k, "kubectl.kubernetes.io/") ||
			strings.HasPrefix(k, "argocd.argoproj.io/") ||
			strings.HasPrefix(k, "helm.sh/") {
			continue
		}
		filtered[k] = v
	}
	return filtered
}
