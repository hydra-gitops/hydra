package entity

import (
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// IsKubernetesDefaultsStaticFilename matches static files merged by ChildApp whose name is kubernetes-defaults-*.yaml.
func IsKubernetesDefaultsStaticFilename(base string) bool {
	return strings.HasPrefix(base, "kubernetes-defaults-") && strings.HasSuffix(base, ".yaml")
}

// splitRawKubernetesDefaultsDocs splits a multi-document YAML file on top-level `---` separators.
func splitRawKubernetesDefaultsDocs(fileContent []byte) []string {
	s := strings.TrimSpace(string(fileContent))
	if s == "" {
		return nil
	}
	s = strings.TrimPrefix(s, "---")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	chunks := strings.Split(s, "\n---\n")
	out := make([]string, 0, len(chunks))
	for _, c := range chunks {
		c = strings.TrimSpace(c)
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

// FilterKubernetesDefaultsBody removes YAML documents whose entity ID already exists in existingIds
// (typically IDs from the Helm chart and other static manifests). Returns empty when all documents are dropped.
func FilterKubernetesDefaultsBody(
	l log.Logger,
	relPath string,
	fileContent []byte,
	existingIds sets.Set[types.Id],
	key types.EntityKeyUnstructured,
) ([]byte, error) {
	if len(existingIds) == 0 {
		return fileContent, nil
	}
	docs := splitRawKubernetesDefaultsDocs(fileContent)
	if len(docs) == 0 {
		return nil, nil
	}
	var kept []string
	for _, doc := range docs {
		wrapped := types.YamlString("# Source: " + relPath + "\n" + doc)
		ents, err := NewEntitiesFromYaml(l, wrapped, key)
		if err != nil {
			return nil, err
		}
		if len(ents.Items) == 0 {
			continue
		}
		id, err := ents.Items[0].Id()
		if err != nil {
			continue
		}
		if existingIds.Has(id) {
			l.DebugLog(logIdEntities,
				"omit kubernetes-defaults document (duplicate id {id}) from {path}",
				log.String("id", string(id)),
				log.String("path", relPath))
			continue
		}
		kept = append(kept, doc)
	}
	if len(kept) == 0 {
		return nil, nil
	}
	return []byte(strings.Join(kept, "\n---\n")), nil
}
