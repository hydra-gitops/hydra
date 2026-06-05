package commands

import (
	"fmt"
	"os"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/sops"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	hyaml "hydra-gitops.org/hydra/hydra-go/core/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	kyaml "sigs.k8s.io/yaml"
)

// SopsDecryptor decrypts a SOPS-encrypted YAML string and returns the plaintext.
// Used as fallback when the entity has no AbsPath or the file does not exist.
// In tests a mock is used.
type SopsDecryptor func(yaml types.YamlString) (types.YamlString, error)

const sopsSecretKind = "SopsSecret"

// sopsSecretManagedAnnotation marks a plain Secret so sops-secrets-operator can adopt
// a pre-existing child Secret after bootstrap apply (no ownerReferences on CLI-created Secrets).
const (
	sopsSecretManagedAnnotationKey   = "sopssecret/managed"
	sopsSecretManagedAnnotationValue = "true"
)

// ExpandSopsSecretsForUninstall finds all SopsSecret entities and creates
// additional v1/Secret entity stubs so that the uninstall flow includes the
// secrets originally derived from SopsSecrets during bootstrap.
// Unlike ConvertSopsSecretsToSecrets this does NOT require SOPS decryption
// because only the template names and namespaces are needed for matching,
// and SOPS only encrypts data/stringData fields.
func ExpandSopsSecretsForUninstall(
	l log.Logger,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
) (entity.Entities, error) {
	var additionalSecrets []entity.Entity

	for _, e := range entities.Items {
		kind, err := e.Kind()
		if err != nil {
			return entity.Entities{}, err
		}
		if string(kind) != sopsSecretKind {
			continue
		}

		u, ok := e.Unstructured(key)
		if !ok {
			continue
		}

		if isBackupSopsSecret(u) {
			name, _ := e.Name()
			l.DebugLog(logIdCommands, "uninstall: skipping backup SopsSecret {name}",
				log.String("name", string(name)))
			continue
		}

		namespace := u.GetNamespace()

		templates, err := extractSecretTemplates(u.Object)
		if err != nil {
			name, _ := e.Name()
			l.DebugLog(logIdCommands, "uninstall: skipping SopsSecret {name}: {err}",
				log.String("name", string(name)),
				log.String("err", err.Error()))
			continue
		}

		for _, tmpl := range templates {
			secretU := buildSecretUnstructured(tmpl, namespace)
			gvk := types.NewGVK(types.Group(""), types.Version("v1"), types.Kind("Secret"))

			b := entity.NewEntityBuilder().
				WithGVK(gvk).
				WithName(types.Name(secretU.GetName())).
				WithUnstructured(key, secretU)

			if namespace != "" {
				b = b.WithNamespace(types.Namespace(namespace)).
					WithNamespaced(types.NamespacedNo)
			}

			l.DebugLog(logIdCommands, "uninstall: derived Secret {name} in namespace {ns} from SopsSecret",
				log.String("name", secretU.GetName()),
				log.String("ns", namespace))

			se, buildErr := b.Build()
			if buildErr != nil {
				return entity.Entities{}, buildErr
			}
			if parentAppIds, perr := e.AppIds(); perr == nil && len(parentAppIds) > 0 {
				se, buildErr = se.Modify(func(br entity.EntityBuilder) entity.EntityBuilder {
					br = br.WithAppIds(parentAppIds)
					return br.WithAppId(parentAppIds[0])
				})
				if buildErr != nil {
					return entity.Entities{}, buildErr
				}
			}
			additionalSecrets = append(additionalSecrets, se)
		}
	}

	if len(additionalSecrets) == 0 {
		return entities, nil
	}

	l.Info(logIdCommands, "uninstall: derived {count} Secrets from SopsSecret CRs",
		log.Int("count", len(additionalSecrets)))

	return entities.Append(entity.Entities{Items: additionalSecrets})
}

// AppendDerivedSopsSecretsForUninstall copies v1/Secret stubs from expandedOffline into
// renderedSelected when a SopsSecret in renderedSelected defines the same secretTemplates
// and that Secret was added by ExpandSopsSecretsForUninstall (present in expandedOffline
// but not in preExpandOffline). This lets cluster uninstall run ExpandSopsSecretsForUninstall
// once on the full offline render while still including derived Secrets for the selected-app
// set from RenderCluster (online pipeline).
func AppendDerivedSopsSecretsForUninstall(
	renderedSelected entity.Entities,
	preExpandOffline entity.Entities,
	expandedOffline entity.Entities,
	key types.EntityKeyUnstructured,
) (entity.Entities, error) {
	preIds, err := CollectEntityIds(preExpandOffline)
	if err != nil {
		return entity.Entities{}, err
	}
	postIds, err := CollectEntityIds(expandedOffline)
	if err != nil {
		return entity.Entities{}, err
	}
	addedIds := sets.New[types.Id]()
	for _, id := range postIds.UnsortedList() {
		if !preIds.Has(id) {
			addedIds.Insert(id)
		}
	}
	if addedIds.Len() == 0 {
		return renderedSelected, nil
	}

	idToEntity := make(map[types.Id]entity.Entity, len(expandedOffline.Items))
	for _, e := range expandedOffline.Items {
		id, idErr := e.Id()
		if idErr != nil {
			return entity.Entities{}, idErr
		}
		idToEntity[id] = e
	}

	var toAppend []entity.Entity
	seen := sets.New[types.Id]()
	for _, e := range renderedSelected.Items {
		kind, kindErr := e.Kind()
		if kindErr != nil {
			return entity.Entities{}, kindErr
		}
		if string(kind) != sopsSecretKind {
			continue
		}
		u, ok := e.Unstructured(key)
		if !ok {
			continue
		}
		if isBackupSopsSecret(u) {
			continue
		}
		ns := u.GetNamespace()
		templates, tmplErr := extractSecretTemplates(u.Object)
		if tmplErr != nil {
			continue
		}
		for _, tmpl := range templates {
			secretU := buildSecretUnstructured(tmpl, ns)
			gvk := types.NewGVK(types.Group(""), types.Version("v1"), types.Kind("Secret"))
			b := entity.NewEntityBuilder().
				WithGVK(gvk).
				WithName(types.Name(secretU.GetName())).
				WithUnstructured(key, secretU)
			if ns != "" {
				b = b.WithNamespace(types.Namespace(ns)).
					WithNamespaced(types.NamespacedNo)
			}
			stub, buildErr := b.Build()
			if buildErr != nil {
				return entity.Entities{}, buildErr
			}
			id, idErr := stub.Id()
			if idErr != nil {
				return entity.Entities{}, idErr
			}
			if !addedIds.Has(id) {
				continue
			}
			if seen.Has(id) {
				continue
			}
			full, ok := idToEntity[id]
			if !ok {
				continue
			}
			seen.Insert(id)
			toAppend = append(toAppend, full)
		}
	}
	if len(toAppend) == 0 {
		return renderedSelected, nil
	}
	return renderedSelected.Append(entity.Entities{Items: toAppend})
}

// ConvertSopsSecretsToSecrets finds all SopsSecret entities, decrypts them,
// and returns the original entities plus additional plain v1/Secret entities
// derived from each SopsSecret's spec.secretTemplates.
// Backup SopsSecrets (annotated with hydra-gitops.org/hydra-backup) are skipped -- they are
// restored exclusively by "hydra gitops backup restore".
func ConvertSopsSecretsToSecrets(
	l log.Logger,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	decrypt SopsDecryptor,
) (entity.Entities, error) {
	var additionalSecrets []entity.Entity

	for _, e := range entities.Items {
		kind, err := e.Kind()
		if err != nil {
			return entity.Entities{}, err
		}
		if string(kind) != sopsSecretKind {
			continue
		}

		u, ok := e.Unstructured(key)
		if !ok {
			continue
		}

		if isBackupSopsSecret(u) {
			name, _ := e.Name()
			l.DebugLog(logIdCommands, "bootstrap: skipping backup SopsSecret {name}",
				log.String("name", string(name)))
			continue
		}

		secrets, err := convertSopsSecret(l, e, u, key, decrypt)
		if err != nil {
			name, _ := e.Name()
			return entity.Entities{}, fmt.Errorf("failed to convert SopsSecret %s: %w", name, err)
		}
		additionalSecrets = append(additionalSecrets, secrets...)
	}

	if len(additionalSecrets) == 0 {
		return entities, nil
	}

	l.Info(logIdCommands, "bootstrap: created {count} plain Secrets from SopsSecret CRs",
		log.Int("count", len(additionalSecrets)))

	return entities.Append(entity.Entities{Items: additionalSecrets})
}

// convertSopsSecret decrypts a single SopsSecret and returns one entity.Entity
// per entry in spec.secretTemplates.
// If the entity has an AbsPath pointing to an existing file, sops decrypts it
// directly (preserving original bytes to avoid MAC mismatch). Otherwise it
// falls back to in-memory decryption via the decrypt function.
func convertSopsSecret(
	l log.Logger,
	e entity.Entity,
	u unstructured.Unstructured,
	key types.EntityKeyUnstructured,
	decrypt SopsDecryptor,
) ([]entity.Entity, error) {
	var decryptedYaml types.YamlString

	absPath, _ := e.AbsPath()
	if absPath != "" {
		if _, err := os.Stat(string(absPath)); err == nil {
			l.DebugLog(logIdCommands, "bootstrap: decrypting SopsSecret from {absPath}",
				log.String("absPath", string(absPath)))
			var err error
			decryptedYaml, err = sops.DecryptSopsFile(string(absPath))
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt SopsSecret (%s): %w", absPath, err)
			}
		} else {
			l.DebugLog(logIdCommands, "bootstrap: absPath {absPath} does not exist, falling back to in-memory decryption",
				log.String("absPath", string(absPath)))
			yamlBytes, err := kyaml.Marshal(u.Object)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize SopsSecret to YAML: %w", err)
			}
			decryptedYaml, err = decrypt(types.YamlString(yamlBytes))
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt SopsSecret (file not found: %s): %w", absPath, err)
			}
		}
	} else {
		l.DebugLog(logIdCommands, "bootstrap: no absPath on SopsSecret entity, falling back to in-memory decryption")
		yamlBytes, err := kyaml.Marshal(u.Object)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize SopsSecret to YAML: %w", err)
		}
		decryptedYaml, err = decrypt(types.YamlString(yamlBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt SopsSecret (no absPath available): %w", err)
		}
	}

	decrypted, err := hyaml.YamlToUnstructured(decryptedYaml)
	if err != nil {
		return nil, fmt.Errorf("failed to parse decrypted SopsSecret: %w", err)
	}

	namespace := u.GetNamespace()

	templates, err := extractSecretTemplates(decrypted.Object)
	if err != nil {
		return nil, err
	}

	var result []entity.Entity
	for _, tmpl := range templates {
		secretU := buildSecretUnstructured(tmpl, namespace)
		gvk := types.NewGVK(types.Group(""), types.Version("v1"), types.Kind("Secret"))

		b := entity.NewEntityBuilder().
			WithGVK(gvk).
			WithName(types.Name(secretU.GetName())).
			WithUnstructured(key, secretU)

		if namespace != "" {
			b = b.WithNamespace(types.Namespace(namespace)).
				WithNamespaced(types.NamespacedNo)
		}

		l.DebugLog(logIdCommands, "bootstrap: created Secret {name} in namespace {ns} from SopsSecret",
			log.String("name", secretU.GetName()),
			log.String("ns", namespace))

		e, buildErr := b.Build()
		if buildErr != nil {
			return nil, buildErr
		}
		result = append(result, e)
	}

	return result, nil
}

// secretTemplate holds the parsed fields from a SopsSecret spec.secretTemplates entry.
type secretTemplate struct {
	Name        string
	Type        string
	StringData  map[string]any
	Data        map[string]any
	Labels      map[string]string
	Annotations map[string]string
}

// extractSecretTemplates parses spec.secretTemplates from a decrypted SopsSecret.
func extractSecretTemplates(obj map[string]any) ([]secretTemplate, error) {
	spec, ok := obj["spec"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("SopsSecret has no spec field")
	}

	templatesRaw, ok := spec["secretTemplates"]
	if !ok {
		return nil, fmt.Errorf("SopsSecret spec has no secretTemplates field")
	}

	templatesList, ok := templatesRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("SopsSecret spec.secretTemplates is not a list")
	}

	var result []secretTemplate
	for i, raw := range templatesList {
		tmplMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("secretTemplates[%d] is not a map", i)
		}

		tmpl := secretTemplate{}

		if name, ok := tmplMap["name"].(string); ok {
			tmpl.Name = name
		} else {
			return nil, fmt.Errorf("secretTemplates[%d] has no name", i)
		}

		if typ, ok := tmplMap["type"].(string); ok {
			tmpl.Type = typ
		}

		if sd, ok := tmplMap["stringData"].(map[string]any); ok {
			tmpl.StringData = sd
		}

		if d, ok := tmplMap["data"].(map[string]any); ok {
			tmpl.Data = d
		}

		if labels, ok := tmplMap["labels"].(map[string]any); ok {
			tmpl.Labels = toStringMap(labels)
		}

		if annotations, ok := tmplMap["annotations"].(map[string]any); ok {
			tmpl.Annotations = toStringMap(annotations)
		}

		result = append(result, tmpl)
	}

	return result, nil
}

func toStringMap(m map[string]any) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}

// buildSecretUnstructured constructs a plain v1/Secret Unstructured from a secretTemplate.
func buildSecretUnstructured(tmpl secretTemplate, namespace string) unstructured.Unstructured {
	metadata := map[string]any{
		"name": tmpl.Name,
	}
	if namespace != "" {
		metadata["namespace"] = namespace
	}
	if len(tmpl.Labels) > 0 {
		labels := make(map[string]any, len(tmpl.Labels))
		for k, v := range tmpl.Labels {
			labels[k] = v
		}
		metadata["labels"] = labels
	}
	annotations := make(map[string]any, len(tmpl.Annotations)+1)
	for k, v := range tmpl.Annotations {
		annotations[k] = v
	}
	if _, ok := annotations[sopsSecretManagedAnnotationKey]; !ok {
		annotations[sopsSecretManagedAnnotationKey] = sopsSecretManagedAnnotationValue
	}
	metadata["annotations"] = annotations

	obj := map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   metadata,
	}

	if tmpl.Type != "" {
		obj["type"] = tmpl.Type
	}

	if len(tmpl.StringData) > 0 {
		obj["stringData"] = tmpl.StringData
	}

	if len(tmpl.Data) > 0 {
		obj["data"] = tmpl.Data
	}

	return unstructured.Unstructured{Object: obj}
}
