package commands

import (
	"fmt"
	"sort"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/colors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	htypes "hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yaml"
	"k8s.io/apimachinery/pkg/util/sets"
)

type AppSyncStatus struct {
	AppId  htypes.AppId
	InSync bool
}

func BuildAppSyncStatuses(
	selectedAppIds sets.Set[htypes.AppId],
	changedAppIds sets.Set[htypes.AppId],
) []AppSyncStatus {
	appIds := selectedAppIds.UnsortedList()
	sort.Slice(appIds, func(i, j int) bool {
		return appIds[i] < appIds[j]
	})

	statuses := make([]AppSyncStatus, 0, len(appIds))
	for _, appId := range appIds {
		statuses = append(statuses, AppSyncStatus{
			AppId:  appId,
			InSync: !changedAppIds.Has(appId),
		})
	}
	return statuses
}

func ComputeClusterAppStatuses(
	renderedEntities entity.Entities,
	clusterEntities entity.Entities,
	selectedAppIds sets.Set[htypes.AppId],
) ([]AppSyncStatus, error) {
	merged, err := renderedEntities.Merge(
		clusterEntities,
		htypes.KeyTemplateEntity,
		htypes.KeyClusterEntity,
	)
	if err != nil {
		return nil, err
	}

	compareResult, err := merged.Compare(htypes.KeyTemplateEntity, htypes.KeyClusterEntity)
	if err != nil {
		return nil, err
	}

	changedAppIds := sets.New[htypes.AppId]()
	for _, item := range compareResult.LeftOnly.Items {
		markEntityAppIdsAsChanged(item, selectedAppIds, changedAppIds)
	}

	for _, item := range compareResult.Both.Items {
		if clusterStatusEntityYaml(item, htypes.KeyTemplateEntity) == clusterStatusEntityYaml(item, htypes.KeyClusterEntity) {
			continue
		}
		markEntityAppIdsAsChanged(item, selectedAppIds, changedAppIds)
	}

	orphans, err := selectTrackedLiveOnlyResources(compareResult.RightOnly, selectedAppIds)
	if err != nil {
		return nil, err
	}
	for _, orphan := range orphans.Items {
		appId, ok := trackedAppIdForEntity(orphan, htypes.KeyClusterEntity)
		if ok && selectedAppIds.Has(appId) {
			changedAppIds.Insert(appId)
		}
	}

	return BuildAppSyncStatuses(selectedAppIds, changedAppIds), nil
}

func ClusterStatus(
	cluster *hydra.Cluster,
	color htypes.Color,
	appIds sets.Set[htypes.AppId],
	networkMode htypes.HelmNetworkMode,
) error {
	_ = networkMode
	skipRootApps := htypes.SkipRootApps(cluster.ClusterName != htypes.InCluster)
	renderedEntities, _, _, err := RenderCluster(cluster, appIds, "", htypes.CrdModeError, skipRootApps, nil)
	if err != nil {
		return err
	}

	clusterEntities, err := ListClusterAll(cluster, htypes.KeyClusterEntity, false, 0)
	if err != nil {
		return err
	}
	model, err := BuildResourceModel(ResourceModelInput{
		Cluster:         cluster,
		ClusterEntities: &clusterEntities,
		NetworkMode:     htypes.HelmNetworkModeOffline,
		Bootstrap:       htypes.BootstrapNo,
	}, false)
	if err != nil {
		return err
	}
	clusterEntities = model.ClusterEntities()

	statuses, err := ComputeClusterAppStatuses(renderedEntities, clusterEntities, appIds)
	if err != nil {
		return err
	}

	printClusterAppStatuses(statuses, color)
	return nil
}

func clusterStatusEntityYaml(e entity.Entity, key htypes.EntityKeyUnstructured) string {
	u, ok := e.Unstructured(key)
	if !ok {
		return ""
	}
	yamlStr, err := yaml.PrintObject(htypes.KeepServerFieldsNo, nil, &u)
	if err != nil {
		return ""
	}
	return string(yamlStr)
}

func markEntityAppIdsAsChanged(e entity.Entity, selectedAppIds sets.Set[htypes.AppId], changedAppIds sets.Set[htypes.AppId]) {
	entityAppIds, err := e.AppIds()
	if err != nil {
		return
	}
	for _, appId := range entityAppIds {
		if selectedAppIds.Has(appId) {
			changedAppIds.Insert(appId)
		}
	}
}

func trackedAppIdForEntity(e entity.Entity, key htypes.EntityKeyUnstructured) (htypes.AppId, bool) {
	u, ok := e.Unstructured(key)
	if !ok {
		return "", false
	}
	trackingId := u.GetAnnotations()["argocd.argoproj.io/tracking-id"]
	if trackingId == "" {
		return "", false
	}
	app, _, ok := strings.Cut(trackingId, ":")
	if !ok || app == "" {
		return "", false
	}
	return htypes.AppId(app), true
}

func selectTrackedLiveOnlyResources(
	candidates entity.Entities,
	selectedAppIds sets.Set[htypes.AppId],
) (entity.Entities, error) {
	if candidates.Len() == 0 {
		return candidates, nil
	}

	candidates, err := candidates.UnselectAll()
	if err != nil {
		return entity.Entities{}, err
	}

	candidates, err = MarkAsSelectedArgoCdManagedResources(log.Default(), candidates, htypes.KeyClusterEntity, selectedAppIds)
	if err != nil {
		return entity.Entities{}, err
	}

	selected, err := candidates.Selected()
	if err != nil {
		return entity.Entities{}, err
	}

	filtered := make([]entity.Entity, 0, selected.Len())
	for _, item := range selected.Items {
		owners := item.OwnerUids(htypes.KeyClusterEntity)
		if owners != nil && owners.Len() > 0 {
			continue
		}
		filtered = append(filtered, item)
	}

	return entity.NewEntities(filtered)
}

func printClusterAppStatuses(statuses []AppSyncStatus, color htypes.Color) {
	for _, status := range statuses {
		label := "out of sync"
		labelColor := colors.Red
		if status.InSync {
			label = "in sync"
			labelColor = colors.Green
		}

		if color {
			fmt.Printf("%s  %s%s%s\n",
				status.AppId,
				labelColor.String(),
				label,
				colors.Reset.String(),
			)
			continue
		}

		fmt.Printf("%s  %s\n", status.AppId, label)
	}
}
