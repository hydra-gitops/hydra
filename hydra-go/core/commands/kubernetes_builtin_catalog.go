package commands

// Kubernetes minimal built-ins: upstream default Namespaces and default ServiceAccounts only.
// All other bootstrap objects (RBAC, kube-root-ca ConfigMaps, kubernetes Service, etc.) live in
// hydra builtin cluster_defaults presets (see KubernetesClusterDefault*).

type kubernetesCoreBuiltin struct {
	Version   string
	Kind      string
	Namespace string
	Name      string
}

var kubernetesClusterMinimalBuiltins = []kubernetesCoreBuiltin{
	{"v1", "Namespace", "", "default"},
	{"v1", "Namespace", "", "kube-system"},
	{"v1", "Namespace", "", "kube-public"},
	{"v1", "Namespace", "", "kube-node-lease"},
	{"v1", "ServiceAccount", "default", "default"},
	{"v1", "ServiceAccount", "kube-system", "default"},
	{"v1", "ServiceAccount", "kube-public", "default"},
	{"v1", "ServiceAccount", "kube-node-lease", "default"},
}
