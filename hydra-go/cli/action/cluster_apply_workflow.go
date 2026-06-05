package action

// Stable workflow phase ids (match basephase.Builder names in buildApplyPhases).
const (
	WorkflowApplyCRDs        = "apply-crds"
	WorkflowApplyNamespaces  = "apply-namespaces"
	WorkflowRestoreBackups   = "restore-backups"
	WorkflowDisableWebhooks  = "disable-webhooks"
	WorkflowApplyScaleZero   = "apply-scale-zero"
	WorkflowScaleUpWorkloads = "scale-up-workloads"
	WorkflowApplyWebhooks    = "apply-webhooks"
	WorkflowScaleDownOrphans = "scale-down-orphans"
	WorkflowDeleteOrphans    = "delete-orphans"
)

// PlannedWorkflowPhase describes one apply phase before skip resolution.
type PlannedWorkflowPhase struct {
	ID          string
	Description string
	// Optional means the phase is only present when the corresponding behavior is enabled via flags.
	Optional bool
}

// ResolvedWorkflowPhase is PlannedWorkflowPhase plus skip resolution for tests and diagnostics.
type ResolvedWorkflowPhase struct {
	PlannedWorkflowPhase
	Skipped bool
	Reason  string
}

// BuildClusterApplyWorkflowPlan returns the ordered phase list for the given flags (step 1: flags → workflow).
// Optional phases appear only when their flags require them.
func BuildClusterApplyWorkflowPlan(f ClusterApplyFlags) []PlannedWorkflowPhase {
	var out []PlannedWorkflowPhase
	out = append(out, PlannedWorkflowPhase{
		ID: WorkflowApplyCRDs, Description: "applying CRDs", Optional: false,
	})
	out = append(out, PlannedWorkflowPhase{
		ID: WorkflowApplyNamespaces, Description: "applying namespaces", Optional: false,
	})
	if f.BackupRestore && !f.SkipBackupRestore {
		out = append(out, PlannedWorkflowPhase{
			ID: WorkflowRestoreBackups, Description: "restoring backup secrets", Optional: true,
		})
	}
	if f.DisableWebhooks {
		out = append(out, PlannedWorkflowPhase{
			ID: WorkflowDisableWebhooks, Description: "disabling non-ready webhook configurations", Optional: true,
		})
	}
	out = append(out, PlannedWorkflowPhase{
		ID: WorkflowApplyScaleZero, Description: "applying main workload resources", Optional: false,
	})
	if f.ScaleUp {
		out = append(out, PlannedWorkflowPhase{
			ID: WorkflowScaleUpWorkloads, Description: "scaling up workloads in dependency order", Optional: true,
		})
	}
	webhookPhaseDesc := "applying webhook configurations"
	if f.DisableWebhooks {
		webhookPhaseDesc = "enabling webhook configurations in provider dependency order"
	}
	out = append(out, PlannedWorkflowPhase{
		ID: WorkflowApplyWebhooks, Description: webhookPhaseDesc, Optional: false,
	})
	if f.OrphanScaleDown {
		out = append(out, PlannedWorkflowPhase{
			ID: WorkflowScaleDownOrphans, Description: "scaling down orphaned workloads", Optional: true,
		})
	}
	out = append(out, PlannedWorkflowPhase{
		ID: WorkflowDeleteOrphans, Description: "deleting orphaned resources", Optional: false,
	})
	return out
}

// WorkflowPlanIDs returns only phase ids in order.
func WorkflowPlanIDs(plan []PlannedWorkflowPhase) []string {
	out := make([]string, len(plan))
	for i, p := range plan {
		out[i] = p.ID
	}
	return out
}

// MarkClusterApplyWorkflowSkipped resolves skipped vs active phases from a plan and apply state (step 2: workflow + entities → skip).
func MarkClusterApplyWorkflowSkipped(plan []PlannedWorkflowPhase, s *applyState) []ResolvedWorkflowPhase {
	out := make([]ResolvedWorkflowPhase, 0, len(plan))
	for _, p := range plan {
		skipped, reason := resolveWorkflowPhaseSkipped(p.ID, s)
		out = append(out, ResolvedWorkflowPhase{
			PlannedWorkflowPhase: p,
			Skipped:              skipped,
			Reason:               reason,
		})
	}
	return out
}

func resolveWorkflowPhaseSkipped(id string, s *applyState) (bool, string) {
	if s == nil {
		return true, "nil state"
	}
	switch id {
	case WorkflowApplyCRDs:
		if s.crds.Len() == 0 {
			return true, "no CRDs"
		}
		if s.ops == nil {
			return true, "no operations"
		}
		crdOps, err := s.ops.FilterCRDs()
		if err != nil {
			return false, ""
		}
		if !crdOps.HasMutatingWork() {
			return true, "no CRD changes"
		}
		return false, ""
	case WorkflowApplyNamespaces:
		if s.namespaces.Len() == 0 {
			return true, "no namespaces"
		}
		if s.ops == nil {
			return true, "no operations"
		}
		nsOps, err := s.ops.FilterNamespaces()
		if err != nil {
			return false, ""
		}
		if !nsOps.HasMutatingWork() {
			return true, "no namespace changes"
		}
		return false, ""
	case WorkflowRestoreBackups:
		// Phase only exists when backup restore is enabled; skip is decided at runtime by restore candidates.
		return false, ""
	case WorkflowDisableWebhooks:
		we, _, err := splitWebhooks(s.nonCrds)
		if err != nil {
			return false, ""
		}
		if we.Len() == 0 {
			return true, "no webhook configurations"
		}
		return false, ""
	case WorkflowApplyScaleZero:
		phaseEntities := s.nonCrds
		if !s.flags.DisableWebhooks {
			_, nonWh, err := splitApplyWebhooks(s)
			if err != nil {
				return false, ""
			}
			phaseEntities = nonWh
		}
		if phaseEntities.Len() == 0 {
			return true, "no resources to apply"
		}
		if s.ops == nil {
			return true, "no operations"
		}
		mainOps, err := s.ops.FilterMainWorkload()
		if err != nil {
			return false, ""
		}
		if s.flags.DisableWebhooks {
			mainOps, err = s.ops.FilterPostNamespaceApply()
			if err != nil {
				return false, ""
			}
		}
		if !mainOps.HasMutatingWork() {
			return true, "no workload changes"
		}
		return false, ""
	case WorkflowScaleUpWorkloads:
		return false, ""
	case WorkflowApplyWebhooks:
		we, _, err := splitApplyWebhooks(s)
		if err != nil {
			return false, ""
		}
		if we.Len() == 0 {
			return true, "no webhook configurations"
		}
		if s.ops == nil {
			return true, "no operations"
		}
		if s.flags.DisableWebhooks {
			return false, ""
		}
		whOps, err := s.ops.FilterWebhooks()
		if err != nil {
			return false, ""
		}
		if !whOps.HasMutatingWork() {
			return true, "no webhook changes"
		}
		return false, ""
	case WorkflowScaleDownOrphans:
		return false, ""
	case WorkflowDeleteOrphans:
		if s.orphans.Len() == 0 {
			return true, "no orphaned resources"
		}
		return false, ""
	default:
		return false, ""
	}
}
