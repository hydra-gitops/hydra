package action

import (
	"fmt"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/colors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

const optionalBehaviorsTablePrefix = "  "

// clusterApplyBootstrapBundleHintMessage is logged after the optional-behaviors table when --bootstrap is not set.
const clusterApplyBootstrapBundleHintMessage = "Tip: use --bootstrap to enable all optional apply behaviors for this run; use matching --no-* flags to opt out of individual behaviors."

func shouldLogBootstrapBundleHint(f ClusterApplyFlags) bool {
	return f.Bootstrap == types.BootstrapNo
}

// logClusterApplyOptionalBehaviorsTable prints a fixed table of optional apply behaviors (the same
// flags that --bootstrap can imply), their effective state for this run, and the CLI flags to enable
// or disable each behavior. Shown during cluster apply immediately after resolved app IDs are logged
// and before rendering templates or contacting the cluster.
func logClusterApplyOptionalBehaviorsTable(l log.Logger, color bool, f ClusterApplyFlags) {
	// Emit header and table in one log record so both go through the same slog/mpb writer.
	// Using fmt.Print for the table used stdout while InfoLog used stderr (via ProgressBars); that
	// split could hide the table or interleave badly with the footer progress renderer.
	table := renderClusterApplyOptionalBehaviorsTable(color, f)
	l.Info(logIdAction, "optional apply behaviors:\n"+table)
	if shouldLogBootstrapBundleHint(f) {
		l.Info(logIdAction, clusterApplyBootstrapBundleHintMessage)
	}
}

func padRightASCII(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func formatBoolPlain(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func formatActiveCell(plain string, color bool, isBool bool) string {
	if !color {
		return plain
	}
	if isBool {
		switch plain {
		case "true":
			return colors.Green.String() + "true" + colors.Reset.String()
		case "false":
			return colors.LightYellow.String() + "false" + colors.Reset.String()
		}
	}
	return colors.Cyan.String() + plain + colors.Reset.String()
}

type optionalBehaviorRow struct {
	option  string
	active  string
	isBool  bool
	enable  string
	disable string
}

// renderClusterApplyOptionalBehaviorsTable renders the optional-behaviors table (including a trailing newline).
func renderClusterApplyOptionalBehaviorsTable(color bool, f ClusterApplyFlags) string {
	sw := string(f.EffectiveSyncWindow)
	if sw == "" {
		sw = string(types.ClusterApplySyncWindowDefault)
	}
	rows := []optionalBehaviorRow{
		{"Decode SOPS", formatBoolPlain(f.SopsDecode), true, "--sops-decode", "--no-sops-decode"},
		{"Down-scaled apply", formatBoolPlain(f.DownScaled), true, "--down-scaled", "--no-down-scaled"},
		{"Scale-up", formatBoolPlain(f.ScaleUp), true, "--scale-up", "--no-scale-up"},
		{"Orphan scale-down", formatBoolPlain(f.OrphanScaleDown), true, "--orphan-scale-down", "--no-orphan-scale-down"},
		{"ArgoCD sync policy", sw, false, "--sync=<mode>", "--sync=default (overrides bootstrap keep-or-prevent)"},
		{"Bootstrap guard", formatBoolPlain(f.BootstrapGuard), true, "--bootstrap-guard", "--no-bootstrap-guard"},
		{"Bootstrap clones", formatBoolPlain(f.BootstrapClones), true, "--bootstrap-clones", "--no-bootstrap-clones"},
		{"Backup restore", formatBoolPlain(f.BackupRestore), true, "--backup-restore", "--no-backup-restore"},
		{"Webhook downscale/enable", formatBoolPlain(f.DisableWebhooks), true, "--disable-webhooks", "--no-disable-webhooks"},
	}

	wOpt, wAct, wEna, wDis := len("Option"), len("active"), len("enable"), len("disable")
	for _, r := range rows {
		if len(r.option) > wOpt {
			wOpt = len(r.option)
		}
		if len(r.active) > wAct {
			wAct = len(r.active)
		}
		if len(r.enable) > wEna {
			wEna = len(r.enable)
		}
		if len(r.disable) > wDis {
			wDis = len(r.disable)
		}
	}

	gap := "  "
	var b strings.Builder
	headerCell := func(plain string, w int) string {
		if !color {
			return padRightASCII(plain, w)
		}
		return colors.BoldWhite() + plain + colors.Reset.String() + strings.Repeat(" ", w-len(plain))
	}
	fmt.Fprintf(&b, "%s%s%s%s%s%s%s%s\n",
		optionalBehaviorsTablePrefix, headerCell("Option", wOpt), gap,
		headerCell("active", wAct), gap,
		headerCell("enable", wEna), gap,
		headerCell("disable", wDis))

	for _, r := range rows {
		activeCell := formatActiveCell(r.active, color, r.isBool)
		plainLen := len(r.active)
		padAfterActive := wAct - plainLen
		if padAfterActive < 0 {
			padAfterActive = 0
		}
		activePadded := activeCell + strings.Repeat(" ", padAfterActive)
		fmt.Fprintf(&b, "%s%s%s%s%s%s%s%s\n",
			optionalBehaviorsTablePrefix, padRightASCII(r.option, wOpt), gap,
			activePadded, gap,
			padRightASCII(r.enable, wEna), gap,
			padRightASCII(r.disable, wDis))
	}
	return b.String()
}
