package commands

import "fmt"

func PhaseMessage(current, total int, description string, skipped bool) string {
	if skipped {
		return fmt.Sprintf("phase %d/%d: %s (skipped)", current, total, description)
	}
	return fmt.Sprintf("phase %d/%d: %s", current, total, description)
}

// PhaseMessageWithID appends a stable workflow id in parentheses after the phase line.
func PhaseMessageWithID(current, total int, description string, skipped bool, workflowID string) string {
	base := PhaseMessage(current, total, description, skipped)
	if workflowID == "" {
		return base
	}
	return fmt.Sprintf("%s (%s)", base, workflowID)
}

func DryRunPrefix(dryRun bool) string {
	if dryRun {
		return "[dry-run] "
	}
	return ""
}
