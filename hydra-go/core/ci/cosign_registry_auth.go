package ci

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	cosignopts "github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"helm.sh/helm/v4/pkg/cli"
	orasauth "oras.land/oras-go/v2/registry/remote/auth"
	orascredentials "oras.land/oras-go/v2/registry/remote/credentials"
)

func resolveCosignRegistryOptions(refOrRegistry string) (cosignopts.RegistryOptions, error) {
	serverAddress, err := registryServerAddress(refOrRegistry)
	if err != nil {
		return cosignopts.RegistryOptions{}, err
	}

	store, err := openRegistryCredentialStore()
	if err != nil {
		return cosignopts.RegistryOptions{}, err
	}
	if store == nil {
		return cosignopts.RegistryOptions{}, nil
	}

	cred, err := store.Get(context.Background(), serverAddress)
	if err != nil {
		return cosignopts.RegistryOptions{}, fmt.Errorf("resolve registry credentials for %s: %w", serverAddress, err)
	}
	if cred == orasauth.EmptyCredential {
		return cosignopts.RegistryOptions{}, nil
	}

	return cosignopts.RegistryOptions{
		AuthConfig: authn.AuthConfig{
			Username:      cred.Username,
			Password:      cred.Password,
			IdentityToken: cred.RefreshToken,
			RegistryToken: cred.AccessToken,
		},
	}, nil
}

func openRegistryCredentialStore() (orascredentials.Store, error) {
	storeOpts := orascredentials.StoreOptions{
		AllowPlaintextPut:        true,
		DetectDefaultNativeStore: false,
	}

	var stores []orascredentials.Store

	settings := cli.New()
	helmStore, err := openSpecificRegistryCredentialStore(settings.RegistryConfig, storeOpts)
	if err != nil {
		return nil, fmt.Errorf("open Helm registry credentials: %w", err)
	}
	if helmStore != nil {
		stores = append(stores, helmStore)
	}

	dockerStore, err := openDockerRegistryCredentialStore(storeOpts)
	if err != nil {
		return nil, fmt.Errorf("open Docker registry credentials: %w", err)
	}
	if dockerStore != nil {
		stores = append(stores, dockerStore)
	}

	switch len(stores) {
	case 0:
		return nil, nil
	case 1:
		return stores[0], nil
	default:
		return orascredentials.NewStoreWithFallbacks(stores[0], stores[1:]...), nil
	}
}

func openSpecificRegistryCredentialStore(path string, opts orascredentials.StoreOptions) (orascredentials.Store, error) {
	store, err := orascredentials.NewStore(path, opts)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return store, nil
}

func openDockerRegistryCredentialStore(opts orascredentials.StoreOptions) (orascredentials.Store, error) {
	store, err := orascredentials.NewStoreFromDocker(opts)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return store, nil
}

func registryServerAddress(refOrRegistry string) (string, error) {
	ref := strings.TrimSpace(strings.TrimPrefix(refOrRegistry, "oci://"))
	if ref == "" {
		return "", fmt.Errorf("resolve registry credentials: empty registry reference")
	}
	host := ref
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("resolve registry credentials: invalid registry reference %q", refOrRegistry)
	}
	return orascredentials.ServerAddressFromRegistry(host), nil
}
