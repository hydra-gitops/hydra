# Cluster Preset Matches (Golden Tests)

The **`entityIds`** in the `*.given.yaml` files may come from the inventory of an **empty** cluster.

Run locally (with a valid `KUBECONFIG` pointing at that cluster):

```bash
./hydra.sh cluster list in-cluster --skip-owner-refs --exclude 'gvk=="events.k8s.io/v1/Event"' > provider.txt
```

The output contains **one Hydra entity ID per line** (same format as in the fixtures). You can copy or adjust those values for `entityIds` in a new or existing `*.given.yaml`.

Notes:

- The golden tests expect **exactly one** matching cluster-defaults preset per entity (1:1). After import, curate the list or adjust presets until the test passes.
- Regenerate the golden file: see the comment in `cluster_preset_matches_golden_test.go` or the `#` lines in the respective `*.expected.yaml`.
- **`talos`**: from `talos.txt` in the workspace root (1:1 IDs, builtin defaults only; among others `coredns` + `kubernetes` active). Regenerate with: `HYDRA_REGEN_TALOS_PRESET_GOLD=1 go test -count=1 -run TestRegenerateTalosClusterPresetFromRepoRootTalos ./hydra` (from `hydra/hydra-go/core`).
