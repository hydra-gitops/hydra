package k8s

import "os"

// FlushProgressLogBeforeFooter flushes stderr so status lines written before a progress bar appears stay above the bar.
func FlushProgressLogBeforeFooter() {
	_ = os.Stderr.Sync()
}

// TruncateFooterDetail shortens a resource id for footer display.
func TruncateFooterDetail(s string) string {
	const max = 96
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
