package types

// Kubernetes Group
const (
	KubernetesGroupCore                       Group = ""
	KubernetesGroupAdmissionregistrationK8sIo Group = "admissionregistration.k8s.io"
	KubernetesGroupApiextensionsK8sIo         Group = "apiextensions.k8s.io"
	KubernetesGroupApiregistrationK8sIo       Group = "apiregistration.k8s.io"
	KubernetesGroupApps                       Group = "apps"
	KubernetesGroupAuthenticationK8sIo        Group = "authentication.k8s.io"
	KubernetesGroupAuthorizationK8sIo         Group = "authorization.k8s.io"
	KubernetesGroupAutoscaling                Group = "autoscaling"
	KubernetesGroupBatch                      Group = "batch"
	KubernetesGroupCertificatesK8sIo          Group = "certificates.k8s.io"
	KubernetesGroupCoordinationK8sIo          Group = "coordination.k8s.io"
	KubernetesGroupDiscoveryK8sIo             Group = "discovery.k8s.io"
	KubernetesGroupEventsK8sIo                Group = "events.k8s.io"
	KubernetesGroupFlowcontrolApiserverK8sIo  Group = "flowcontrol.apiserver.k8s.io"
	KubernetesGroupNetworkingK8sIo            Group = "networking.k8s.io"
	KubernetesGroupNodeK8sIo                  Group = "node.k8s.io"
	KubernetesGroupPolicy                     Group = "policy"
	KubernetesGroupRbacAuthorizationK8sIo     Group = "rbac.authorization.k8s.io"
	KubernetesGroupSchedulingK8sIo            Group = "scheduling.k8s.io"
	KubernetesGroupStorageK8sIo               Group = "storage.k8s.io"
)

// Kubernetes Version
const (
	KubernetesVersionV1 Version = "v1"
	KubernetesVersionV2 Version = "v2"
)

// Kubernetes Resource
const (
	KubernetesResourceapiservices                       Resource = "apiservices"
	KubernetesResourcebindings                          Resource = "bindings"
	KubernetesResourcecertificatesigningrequests        Resource = "certificatesigningrequests"
	KubernetesResourceclusterrolebindings               Resource = "clusterrolebindings"
	KubernetesResourceclusterroles                      Resource = "clusterroles"
	KubernetesResourcecomponentstatuses                 Resource = "componentstatuses"
	KubernetesResourceconfigmaps                        Resource = "configmaps"
	KubernetesResourcecontrollerrevisions               Resource = "controllerrevisions"
	KubernetesResourcecronjobs                          Resource = "cronjobs"
	KubernetesResourcecsidrivers                        Resource = "csidrivers"
	KubernetesResourcecsinodes                          Resource = "csinodes"
	KubernetesResourcecsistoragecapacities              Resource = "csistoragecapacities"
	KubernetesResourcecustomresourcedefinitions         Resource = "customresourcedefinitions"
	KubernetesResourcedaemonsets                        Resource = "daemonsets"
	KubernetesResourcedeployments                       Resource = "deployments"
	KubernetesResourceendpoints                         Resource = "endpoints"
	KubernetesResourceendpointslices                    Resource = "endpointslices"
	KubernetesResourceevents                            Resource = "events"
	KubernetesResourceflowschemas                       Resource = "flowschemas"
	KubernetesResourcehorizontalpodautoscalers          Resource = "horizontalpodautoscalers"
	KubernetesResourceingressclasses                    Resource = "ingressclasses"
	KubernetesResourceingresses                         Resource = "ingresses"
	KubernetesResourcejobs                              Resource = "jobs"
	KubernetesResourceleases                            Resource = "leases"
	KubernetesResourcelimitranges                       Resource = "limitranges"
	KubernetesResourcelocalsubjectaccessreviews         Resource = "localsubjectaccessreviews"
	KubernetesResourcemutatingwebhookconfigurations     Resource = "mutatingwebhookconfigurations"
	KubernetesResourcenamespaces                        Resource = "namespaces"
	KubernetesResourcenetworkpolicies                   Resource = "networkpolicies"
	KubernetesResourcenodes                             Resource = "nodes"
	KubernetesResourcepersistentvolumeclaims            Resource = "persistentvolumeclaims"
	KubernetesResourcepersistentvolumes                 Resource = "persistentvolumes"
	KubernetesResourcepoddisruptionbudgets              Resource = "poddisruptionbudgets"
	KubernetesResourcepods                              Resource = "pods"
	KubernetesResourcepodtemplates                      Resource = "podtemplates"
	KubernetesResourcepriorityclasses                   Resource = "priorityclasses"
	KubernetesResourceprioritylevelconfigurations       Resource = "prioritylevelconfigurations"
	KubernetesResourcereplicasets                       Resource = "replicasets"
	KubernetesResourcereplicationcontrollers            Resource = "replicationcontrollers"
	KubernetesResourceresourcequotas                    Resource = "resourcequotas"
	KubernetesResourcerolebindings                      Resource = "rolebindings"
	KubernetesResourceroles                             Resource = "roles"
	KubernetesResourceruntimeclasses                    Resource = "runtimeclasses"
	KubernetesResourcesecrets                           Resource = "secrets"
	KubernetesResourceselfsubjectaccessreviews          Resource = "selfsubjectaccessreviews"
	KubernetesResourceselfsubjectreviews                Resource = "selfsubjectreviews"
	KubernetesResourceselfsubjectrulesreviews           Resource = "selfsubjectrulesreviews"
	KubernetesResourceserviceaccounts                   Resource = "serviceaccounts"
	KubernetesResourceservices                          Resource = "services"
	KubernetesResourcestatefulsets                      Resource = "statefulsets"
	KubernetesResourcestorageclasses                    Resource = "storageclasses"
	KubernetesResourcesubjectaccessreviews              Resource = "subjectaccessreviews"
	KubernetesResourcetokenreviews                      Resource = "tokenreviews"
	KubernetesResourcevalidatingadmissionpolicies       Resource = "validatingadmissionpolicies"
	KubernetesResourcevalidatingadmissionpolicybindings Resource = "validatingadmissionpolicybindings"
	KubernetesResourcevalidatingwebhookconfigurations   Resource = "validatingwebhookconfigurations"
	KubernetesResourcevolumeattachments                 Resource = "volumeattachments"
)

const (
	KubernetesKindAPIService                       Kind = "APIService"
	KubernetesKindBinding                          Kind = "Binding"
	KubernetesKindCertificateSigningRequest        Kind = "CertificateSigningRequest"
	KubernetesKindClusterRole                      Kind = "ClusterRole"
	KubernetesKindClusterRoleBinding               Kind = "ClusterRoleBinding"
	KubernetesKindComponentStatus                  Kind = "ComponentStatus"
	KubernetesKindConfigMap                        Kind = "ConfigMap"
	KubernetesKindControllerRevision               Kind = "ControllerRevision"
	KubernetesKindCronJob                          Kind = "CronJob"
	KubernetesKindCSIDriver                        Kind = "CSIDriver"
	KubernetesKindCSINode                          Kind = "CSINode"
	KubernetesKindCSIStorageCapacity               Kind = "CSIStorageCapacity"
	KubernetesKindCustomResourceDefinition         Kind = "CustomResourceDefinition"
	KubernetesKindDaemonSet                        Kind = "DaemonSet"
	KubernetesKindDeployment                       Kind = "Deployment"
	KubernetesKindEndpoints                        Kind = "Endpoints"
	KubernetesKindEndpointSlice                    Kind = "EndpointSlice"
	KubernetesKindEvent                            Kind = "Event"
	KubernetesKindFlowSchema                       Kind = "FlowSchema"
	KubernetesKindHorizontalPodAutoscaler          Kind = "HorizontalPodAutoscaler"
	KubernetesKindIngress                          Kind = "Ingress"
	KubernetesKindIngressClass                     Kind = "IngressClass"
	KubernetesKindJob                              Kind = "Job"
	KubernetesKindLease                            Kind = "Lease"
	KubernetesKindLimitRange                       Kind = "LimitRange"
	KubernetesKindLocalSubjectAccessReview         Kind = "LocalSubjectAccessReview"
	KubernetesKindMutatingWebhookConfiguration     Kind = "MutatingWebhookConfiguration"
	KubernetesKindNamespace                        Kind = "Namespace"
	KubernetesKindNetworkPolicy                    Kind = "NetworkPolicy"
	KubernetesKindNode                             Kind = "Node"
	KubernetesKindPersistentVolume                 Kind = "PersistentVolume"
	KubernetesKindPersistentVolumeClaim            Kind = "PersistentVolumeClaim"
	KubernetesKindPod                              Kind = "Pod"
	KubernetesKindPodDisruptionBudget              Kind = "PodDisruptionBudget"
	KubernetesKindPodTemplate                      Kind = "PodTemplate"
	KubernetesKindPriorityClass                    Kind = "PriorityClass"
	KubernetesKindPriorityLevelConfiguration       Kind = "PriorityLevelConfiguration"
	KubernetesKindReplicaSet                       Kind = "ReplicaSet"
	KubernetesKindReplicationController            Kind = "ReplicationController"
	KubernetesKindResourceQuota                    Kind = "ResourceQuota"
	KubernetesKindRole                             Kind = "Role"
	KubernetesKindRoleBinding                      Kind = "RoleBinding"
	KubernetesKindRuntimeClass                     Kind = "RuntimeClass"
	KubernetesKindSecret                           Kind = "Secret"
	KubernetesKindSelfSubjectAccessReview          Kind = "SelfSubjectAccessReview"
	KubernetesKindSelfSubjectReview                Kind = "SelfSubjectReview"
	KubernetesKindSelfSubjectRulesReview           Kind = "SelfSubjectRulesReview"
	KubernetesKindService                          Kind = "Service"
	KubernetesKindServiceAccount                   Kind = "ServiceAccount"
	KubernetesKindStatefulSet                      Kind = "StatefulSet"
	KubernetesKindStorageClass                     Kind = "StorageClass"
	KubernetesKindSubjectAccessReview              Kind = "SubjectAccessReview"
	KubernetesKindTokenReview                      Kind = "TokenReview"
	KubernetesKindValidatingAdmissionPolicy        Kind = "ValidatingAdmissionPolicy"
	KubernetesKindValidatingAdmissionPolicyBinding Kind = "ValidatingAdmissionPolicyBinding"
	KubernetesKindValidatingWebhookConfiguration   Kind = "ValidatingWebhookConfiguration"
	KubernetesKindVolumeAttachment                 Kind = "VolumeAttachment"
)

const (
	KubernetesGvkV1Binding                                                    GVKString = "v1/Binding"
	KubernetesGvkV1ComponentStatus                                            GVKString = "v1/ComponentStatus"
	KubernetesGvkV1ConfigMap                                                  GVKString = "v1/ConfigMap"
	KubernetesGvkV1Endpoints                                                  GVKString = "v1/Endpoints"
	KubernetesGvkV1Event                                                      GVKString = "v1/Event"
	KubernetesGvkV1LimitRange                                                 GVKString = "v1/LimitRange"
	KubernetesGvkV1Namespace                                                  GVKString = "v1/Namespace"
	KubernetesGvkV1Node                                                       GVKString = "v1/Node"
	KubernetesGvkV1PersistentVolumeClaim                                      GVKString = "v1/PersistentVolumeClaim"
	KubernetesGvkV1PersistentVolume                                           GVKString = "v1/PersistentVolume"
	KubernetesGvkV1Pod                                                        GVKString = "v1/Pod"
	KubernetesGvkV1PodTemplate                                                GVKString = "v1/PodTemplate"
	KubernetesGvkV1ReplicationController                                      GVKString = "v1/ReplicationController"
	KubernetesGvkV1ResourceQuota                                              GVKString = "v1/ResourceQuota"
	KubernetesGvkV1Secret                                                     GVKString = "v1/Secret"
	KubernetesGvkV1ServiceAccount                                             GVKString = "v1/ServiceAccount"
	KubernetesGvkV1Service                                                    GVKString = "v1/Service"
	KubernetesGvkAdmissionregistrationK8sIoV1MutatingWebhookConfiguration     GVKString = "admissionregistration.k8s.io/v1/MutatingWebhookConfiguration"
	KubernetesGvkAdmissionregistrationK8sIoV1ValidatingAdmissionPolicy        GVKString = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"
	KubernetesGvkAdmissionregistrationK8sIoV1ValidatingAdmissionPolicyBinding GVKString = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding"
	KubernetesGvkAdmissionregistrationK8sIoV1ValidatingWebhookConfiguration   GVKString = "admissionregistration.k8s.io/v1/ValidatingWebhookConfiguration"
	KubernetesGvkApiextensionsK8sIoV1CustomResourceDefinition                 GVKString = "apiextensions.k8s.io/v1/CustomResourceDefinition"
	KubernetesGvkApiregistrationK8sIoV1APIService                             GVKString = "apiregistration.k8s.io/v1/APIService"
	KubernetesGvkAppsV1ControllerRevision                                     GVKString = "apps/v1/ControllerRevision"
	KubernetesGvkAppsV1DaemonSet                                              GVKString = "apps/v1/DaemonSet"
	KubernetesGvkAppsV1Deployment                                             GVKString = "apps/v1/Deployment"
	KubernetesGvkAppsV1ReplicaSet                                             GVKString = "apps/v1/ReplicaSet"
	KubernetesGvkAppsV1StatefulSet                                            GVKString = "apps/v1/StatefulSet"
	KubernetesGvkAuthenticationK8sIoV1SelfSubjectReview                       GVKString = "authentication.k8s.io/v1/SelfSubjectReview"
	KubernetesGvkAuthenticationK8sIoV1TokenReview                             GVKString = "authentication.k8s.io/v1/TokenReview"
	KubernetesGvkAuthorizationK8sIoV1LocalSubjectAccessReview                 GVKString = "authorization.k8s.io/v1/LocalSubjectAccessReview"
	KubernetesGvkAuthorizationK8sIoV1SelfSubjectAccessReview                  GVKString = "authorization.k8s.io/v1/SelfSubjectAccessReview"
	KubernetesGvkAuthorizationK8sIoV1SelfSubjectRulesReview                   GVKString = "authorization.k8s.io/v1/SelfSubjectRulesReview"
	KubernetesGvkAuthorizationK8sIoV1SubjectAccessReview                      GVKString = "authorization.k8s.io/v1/SubjectAccessReview"
	KubernetesGvkAutoscalingV2HorizontalPodAutoscaler                         GVKString = "autoscaling/v2/HorizontalPodAutoscaler"
	KubernetesGvkBatchV1CronJob                                               GVKString = "batch/v1/CronJob"
	KubernetesGvkBatchV1Job                                                   GVKString = "batch/v1/Job"
	KubernetesGvkCertificatesK8sIoV1CertificateSigningRequest                 GVKString = "certificates.k8s.io/v1/CertificateSigningRequest"
	KubernetesGvkCoordinationK8sIoV1Lease                                     GVKString = "coordination.k8s.io/v1/Lease"
	KubernetesGvkDiscoveryK8sIoV1EndpointSlice                                GVKString = "discovery.k8s.io/v1/EndpointSlice"
	KubernetesGvkEventsK8sIoV1Event                                           GVKString = "events.k8s.io/v1/Event"
	KubernetesGvkFlowcontrolApiserverK8sIoV1FlowSchema                        GVKString = "flowcontrol.apiserver.k8s.io/v1/FlowSchema"
	KubernetesGvkFlowcontrolApiserverK8sIoV1PriorityLevelConfiguration        GVKString = "flowcontrol.apiserver.k8s.io/v1/PriorityLevelConfiguration"
	KubernetesGvkNetworkingK8sIoV1IngressClass                                GVKString = "networking.k8s.io/v1/IngressClass"
	KubernetesGvkNetworkingK8sIoV1Ingress                                     GVKString = "networking.k8s.io/v1/Ingress"
	KubernetesGvkNetworkingK8sIoV1NetworkPolicy                               GVKString = "networking.k8s.io/v1/NetworkPolicy"
	KubernetesGvkNodeK8sIoV1RuntimeClass                                      GVKString = "node.k8s.io/v1/RuntimeClass"
	KubernetesGvkPolicyV1PodDisruptionBudget                                  GVKString = "policy/v1/PodDisruptionBudget"
	KubernetesGvkRbacAuthorizationK8sIoV1ClusterRoleBinding                   GVKString = "rbac.authorization.k8s.io/v1/ClusterRoleBinding"
	KubernetesGvkRbacAuthorizationK8sIoV1ClusterRole                          GVKString = "rbac.authorization.k8s.io/v1/ClusterRole"
	KubernetesGvkRbacAuthorizationK8sIoV1RoleBinding                          GVKString = "rbac.authorization.k8s.io/v1/RoleBinding"
	KubernetesGvkRbacAuthorizationK8sIoV1Role                                 GVKString = "rbac.authorization.k8s.io/v1/Role"
	KubernetesGvkschedulingK8sIoV1PriorityClass                               GVKString = "scheduling.k8s.io/v1/PriorityClass"
	KubernetesGvkstorageK8sIoV1CSIDriver                                      GVKString = "storage.k8s.io/v1/CSIDriver"
	KubernetesGvkstorageK8sIoV1CSINode                                        GVKString = "storage.k8s.io/v1/CSINode"
	KubernetesGvkstorageK8sIoV1CSIStorageCapacity                             GVKString = "storage.k8s.io/v1/CSIStorageCapacity"
	KubernetesGvkstorageK8sIoV1StorageClass                                   GVKString = "storage.k8s.io/v1/StorageClass"
	KubernetesGvkstorageK8sIoV1VolumeAttachment                               GVKString = "storage.k8s.io/v1/VolumeAttachment"
)

func DefaultScopeInfoMap() ScopeInfoMap {
	infos := ScopeInfoMap{}
	for _, info := range k8sResourceInfos {
		infos[info.GVKString()] = info.Clone()
	}
	return infos
}

func defaultScopeInfo(resource Resource, group Group, version Version, namespaced Namespaced, kind Kind) ScopeInfo {
	return ScopeInfo{
		Group:      group,
		Version:    version,
		Resource:   resource,
		Kind:       kind,
		Namespaced: namespaced,
	}
}

var k8sResourceInfos = []ScopeInfo{
	defaultScopeInfo(KubernetesResourcebindings, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindBinding),
	defaultScopeInfo(KubernetesResourcecomponentstatuses, KubernetesGroupCore, KubernetesVersionV1, false, KubernetesKindComponentStatus),
	defaultScopeInfo(KubernetesResourceconfigmaps, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindConfigMap),
	defaultScopeInfo(KubernetesResourceendpoints, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindEndpoints),
	defaultScopeInfo(KubernetesResourceevents, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindEvent),
	defaultScopeInfo(KubernetesResourcelimitranges, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindLimitRange),
	defaultScopeInfo(KubernetesResourcenamespaces, KubernetesGroupCore, KubernetesVersionV1, false, KubernetesKindNamespace),
	defaultScopeInfo(KubernetesResourcenodes, KubernetesGroupCore, KubernetesVersionV1, false, KubernetesKindNode),
	defaultScopeInfo(KubernetesResourcepersistentvolumeclaims, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindPersistentVolumeClaim),
	defaultScopeInfo(KubernetesResourcepersistentvolumes, KubernetesGroupCore, KubernetesVersionV1, false, KubernetesKindPersistentVolume),
	defaultScopeInfo(KubernetesResourcepods, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindPod),
	defaultScopeInfo(KubernetesResourcepodtemplates, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindPodTemplate),
	defaultScopeInfo(KubernetesResourcereplicationcontrollers, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindReplicationController),
	defaultScopeInfo(KubernetesResourceresourcequotas, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindResourceQuota),
	defaultScopeInfo(KubernetesResourcesecrets, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindSecret),
	defaultScopeInfo(KubernetesResourceserviceaccounts, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindServiceAccount),
	defaultScopeInfo(KubernetesResourceservices, KubernetesGroupCore, KubernetesVersionV1, true, KubernetesKindService),
	defaultScopeInfo(KubernetesResourcemutatingwebhookconfigurations, KubernetesGroupAdmissionregistrationK8sIo, KubernetesVersionV1, false, KubernetesKindMutatingWebhookConfiguration),
	defaultScopeInfo(KubernetesResourcevalidatingadmissionpolicies, KubernetesGroupAdmissionregistrationK8sIo, KubernetesVersionV1, false, KubernetesKindValidatingAdmissionPolicy),
	defaultScopeInfo(KubernetesResourcevalidatingadmissionpolicybindings, KubernetesGroupAdmissionregistrationK8sIo, KubernetesVersionV1, false, KubernetesKindValidatingAdmissionPolicyBinding),
	defaultScopeInfo(KubernetesResourcevalidatingwebhookconfigurations, KubernetesGroupAdmissionregistrationK8sIo, KubernetesVersionV1, false, KubernetesKindValidatingWebhookConfiguration),
	defaultScopeInfo(KubernetesResourcecustomresourcedefinitions, KubernetesGroupApiextensionsK8sIo, KubernetesVersionV1, false, KubernetesKindCustomResourceDefinition),
	defaultScopeInfo(KubernetesResourceapiservices, KubernetesGroupApiregistrationK8sIo, KubernetesVersionV1, false, KubernetesKindAPIService),
	defaultScopeInfo(KubernetesResourcecontrollerrevisions, KubernetesGroupApps, KubernetesVersionV1, true, KubernetesKindControllerRevision),
	defaultScopeInfo(KubernetesResourcedaemonsets, KubernetesGroupApps, KubernetesVersionV1, true, KubernetesKindDaemonSet),
	defaultScopeInfo(KubernetesResourcedeployments, KubernetesGroupApps, KubernetesVersionV1, true, KubernetesKindDeployment),
	defaultScopeInfo(KubernetesResourcereplicasets, KubernetesGroupApps, KubernetesVersionV1, true, KubernetesKindReplicaSet),
	defaultScopeInfo(KubernetesResourcestatefulsets, KubernetesGroupApps, KubernetesVersionV1, true, KubernetesKindStatefulSet),
	defaultScopeInfo(KubernetesResourceselfsubjectreviews, KubernetesGroupAuthenticationK8sIo, KubernetesVersionV1, false, KubernetesKindSelfSubjectReview),
	defaultScopeInfo(KubernetesResourcetokenreviews, KubernetesGroupAuthenticationK8sIo, KubernetesVersionV1, false, KubernetesKindTokenReview),
	defaultScopeInfo(KubernetesResourcelocalsubjectaccessreviews, KubernetesGroupAuthorizationK8sIo, KubernetesVersionV1, true, KubernetesKindLocalSubjectAccessReview),
	defaultScopeInfo(KubernetesResourceselfsubjectaccessreviews, KubernetesGroupAuthorizationK8sIo, KubernetesVersionV1, false, KubernetesKindSelfSubjectAccessReview),
	defaultScopeInfo(KubernetesResourceselfsubjectrulesreviews, KubernetesGroupAuthorizationK8sIo, KubernetesVersionV1, false, KubernetesKindSelfSubjectRulesReview),
	defaultScopeInfo(KubernetesResourcesubjectaccessreviews, KubernetesGroupAuthorizationK8sIo, KubernetesVersionV1, false, KubernetesKindSubjectAccessReview),
	defaultScopeInfo(KubernetesResourcehorizontalpodautoscalers, KubernetesGroupAutoscaling, KubernetesVersionV2, true, KubernetesKindHorizontalPodAutoscaler),
	defaultScopeInfo(KubernetesResourcecronjobs, KubernetesGroupBatch, KubernetesVersionV1, true, KubernetesKindCronJob),
	defaultScopeInfo(KubernetesResourcejobs, KubernetesGroupBatch, KubernetesVersionV1, true, KubernetesKindJob),
	defaultScopeInfo(KubernetesResourcecertificatesigningrequests, KubernetesGroupCertificatesK8sIo, KubernetesVersionV1, false, KubernetesKindCertificateSigningRequest),
	defaultScopeInfo(KubernetesResourceleases, KubernetesGroupCoordinationK8sIo, KubernetesVersionV1, true, KubernetesKindLease),
	defaultScopeInfo(KubernetesResourceendpointslices, KubernetesGroupDiscoveryK8sIo, KubernetesVersionV1, true, KubernetesKindEndpointSlice),
	defaultScopeInfo(KubernetesResourceevents, KubernetesGroupEventsK8sIo, KubernetesVersionV1, true, KubernetesKindEvent),
	defaultScopeInfo(KubernetesResourceflowschemas, KubernetesGroupFlowcontrolApiserverK8sIo, KubernetesVersionV1, false, KubernetesKindFlowSchema),
	defaultScopeInfo(KubernetesResourceprioritylevelconfigurations, KubernetesGroupFlowcontrolApiserverK8sIo, KubernetesVersionV1, false, KubernetesKindPriorityLevelConfiguration),
	defaultScopeInfo(KubernetesResourceingressclasses, KubernetesGroupNetworkingK8sIo, KubernetesVersionV1, false, KubernetesKindIngressClass),
	defaultScopeInfo(KubernetesResourceingresses, KubernetesGroupNetworkingK8sIo, KubernetesVersionV1, true, KubernetesKindIngress),
	defaultScopeInfo(KubernetesResourcenetworkpolicies, KubernetesGroupNetworkingK8sIo, KubernetesVersionV1, true, KubernetesKindNetworkPolicy),
	defaultScopeInfo(KubernetesResourceruntimeclasses, KubernetesGroupNodeK8sIo, KubernetesVersionV1, false, KubernetesKindRuntimeClass),
	defaultScopeInfo(KubernetesResourcepoddisruptionbudgets, KubernetesGroupPolicy, KubernetesVersionV1, true, KubernetesKindPodDisruptionBudget),
	defaultScopeInfo(KubernetesResourceclusterrolebindings, KubernetesGroupRbacAuthorizationK8sIo, KubernetesVersionV1, false, KubernetesKindClusterRoleBinding),
	defaultScopeInfo(KubernetesResourceclusterroles, KubernetesGroupRbacAuthorizationK8sIo, KubernetesVersionV1, false, KubernetesKindClusterRole),
	defaultScopeInfo(KubernetesResourcerolebindings, KubernetesGroupRbacAuthorizationK8sIo, KubernetesVersionV1, true, KubernetesKindRoleBinding),
	defaultScopeInfo(KubernetesResourceroles, KubernetesGroupRbacAuthorizationK8sIo, KubernetesVersionV1, true, KubernetesKindRole),
	defaultScopeInfo(KubernetesResourcepriorityclasses, KubernetesGroupSchedulingK8sIo, KubernetesVersionV1, false, KubernetesKindPriorityClass),
	defaultScopeInfo(KubernetesResourcecsidrivers, KubernetesGroupStorageK8sIo, KubernetesVersionV1, false, KubernetesKindCSIDriver),
	defaultScopeInfo(KubernetesResourcecsinodes, KubernetesGroupStorageK8sIo, KubernetesVersionV1, false, KubernetesKindCSINode),
	defaultScopeInfo(KubernetesResourcecsistoragecapacities, KubernetesGroupStorageK8sIo, KubernetesVersionV1, true, KubernetesKindCSIStorageCapacity),
	defaultScopeInfo(KubernetesResourcestorageclasses, KubernetesGroupStorageK8sIo, KubernetesVersionV1, false, KubernetesKindStorageClass),
	defaultScopeInfo(KubernetesResourcevolumeattachments, KubernetesGroupStorageK8sIo, KubernetesVersionV1, false, KubernetesKindVolumeAttachment),
}
