package commands

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/colors"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

func isRefOwnershipUnassignedClusterOnlyMessage(msg string) bool {
	return strings.HasPrefix(msg, RefOwnershipUnassignedClusterOnlyFinding+": resource ") &&
		strings.Contains(msg, RefOwnershipUnassignedClusterOnlyScopeNote)
}

// ReviewFindingMessageGroup returns a stable key for grouping human-readable review output.
// Ref-ownership messages use a constant prefix before ": resource " or ": template owner ";
// missing-key findings are grouped under "missing referenced key".
// Unassigned cluster-only ref ownership findings use RefOwnershipUnassignedClusterOnlyMessageGroupTitle
// so the scope note appears once in the message type line, not per target.
func ReviewFindingMessageGroup(msg string) string {
	if isRefOwnershipUnassignedClusterOnlyMessage(msg) {
		return RefOwnershipUnassignedClusterOnlyMessageGroupTitle
	}
	for _, sep := range []string{": template owner ", ": resource "} {
		if i := strings.Index(msg, sep); i >= 0 {
			return strings.TrimSpace(msg[:i])
		}
	}
	if strings.HasPrefix(msg, "missing referenced key ") {
		return "missing referenced key"
	}
	return msg
}

func reviewFindingDetailAfterGroup(fullMsg, group string) string {
	if isRefOwnershipUnassignedClusterOnlyMessage(fullMsg) {
		return ""
	}
	if fullMsg == group {
		return ""
	}
	if !strings.HasPrefix(fullMsg, group) {
		return strings.TrimSpace(strings.TrimPrefix(fullMsg, group))
	}
	rest := strings.TrimSpace(fullMsg[len(group):])
	rest = strings.TrimPrefix(rest, ":")
	return strings.TrimSpace(rest)
}

// WriteReviewFindingText writes one review finding as human-oriented lines. When useColor is true,
// labels and values use distinct ANSI colors (TTY-oriented).
func WriteReviewFindingText(w io.Writer, finding ReviewFinding, useColor types.Color) error {
	var b strings.Builder
	if bool(useColor) {
		b.WriteString(colors.Red.String())
		b.WriteString("● ")
		b.WriteString("Review finding")
		b.WriteString(colors.Reset.String())
		b.WriteString("\n")
	} else {
		b.WriteString("Review finding\n")
	}

	if bool(useColor) {
		b.WriteString(colors.LightBlue.String())
		b.WriteString("  Target:  ")
		b.WriteString(colors.Reset.String())
		b.WriteString(colors.BoldWhite())
		b.WriteString(string(finding.Target))
		b.WriteString(colors.Reset.String())
		b.WriteString("\n")

		b.WriteString(colors.LightBlue.String())
		b.WriteString("  Message: ")
		b.WriteString(colors.Reset.String())
		b.WriteString(colors.LightYellow.String())
		b.WriteString(finding.Message)
		b.WriteString(colors.Reset.String())
		b.WriteString("\n")

		if len(finding.Sources) > 0 {
			b.WriteString(colors.LightBlue.String())
			b.WriteString("  Sources:")
			b.WriteString(colors.Reset.String())
			b.WriteString("\n")
		}
	} else {
		b.WriteString("  Target:  ")
		b.WriteString(string(finding.Target))
		b.WriteString("\n")
		b.WriteString("  Message: ")
		b.WriteString(finding.Message)
		b.WriteString("\n")
		if len(finding.Sources) > 0 {
			b.WriteString("  Sources:\n")
		}
	}
	if len(finding.Sources) > 0 {
		for _, src := range finding.Sources {
			b.WriteString("    - ")
			if bool(useColor) {
				b.WriteString(colors.LightGray.String())
				b.WriteString(string(src))
				b.WriteString(colors.Reset.String())
			} else {
				b.WriteString(string(src))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	_, err := fmt.Fprint(w, b.String())
	return err
}

// WriteReviewFindingsGroupedText writes review findings grouped by ReviewFindingMessageGroup,
// sorted by group key, then target, then full message.
func WriteReviewFindingsGroupedText(w io.Writer, findings []ReviewFinding, useColor types.Color) error {
	if len(findings) == 0 {
		return nil
	}
	sorted := slices.Clone(findings)
	slices.SortFunc(sorted, func(a, b ReviewFinding) int {
		ga, gb := ReviewFindingMessageGroup(a.Message), ReviewFindingMessageGroup(b.Message)
		if c := cmp.Compare(ga, gb); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Target, b.Target); c != 0 {
			return c
		}
		return cmp.Compare(a.Message, b.Message)
	})

	var b strings.Builder
	var currentGroup string
	for i, f := range sorted {
		g := ReviewFindingMessageGroup(f.Message)
		if g != currentGroup {
			if i > 0 {
				b.WriteString("\n")
			}
			currentGroup = g
			if bool(useColor) {
				b.WriteString(colors.Red.String())
				b.WriteString("● ")
				b.WriteString("Review finding")
				b.WriteString(colors.Reset.String())
				b.WriteString("\n")
				b.WriteString(colors.LightBlue.String())
				b.WriteString("  Message type: ")
				b.WriteString(colors.Reset.String())
				b.WriteString(colors.LightYellow.String())
				b.WriteString(g)
				b.WriteString(colors.Reset.String())
				b.WriteString("\n\n")
			} else {
				b.WriteString("Review finding\n")
				b.WriteString("  Message type: ")
				b.WriteString(g)
				b.WriteString("\n\n")
			}
		}

		if bool(useColor) {
			b.WriteString(colors.LightBlue.String())
			b.WriteString("  - Target:  ")
			b.WriteString(colors.Reset.String())
			b.WriteString(colors.BoldWhite())
			b.WriteString(string(f.Target))
			b.WriteString(colors.Reset.String())
			b.WriteString("\n")
		} else {
			b.WriteString("  - Target:  ")
			b.WriteString(string(f.Target))
			b.WriteString("\n")
		}

		if detail := reviewFindingDetailAfterGroup(f.Message, g); detail != "" {
			line := detail
			label := "    Detail: "
			if g == "missing referenced key" {
				line = f.Message
				label = "    Message: "
			}
			if bool(useColor) {
				b.WriteString(colors.LightBlue.String())
				b.WriteString(label)
				b.WriteString(colors.Reset.String())
				b.WriteString(colors.LightYellow.String())
				b.WriteString(line)
				b.WriteString(colors.Reset.String())
				b.WriteString("\n")
			} else {
				b.WriteString(label)
				b.WriteString(line)
				b.WriteString("\n")
			}
		}

		if len(f.Sources) > 0 {
			if bool(useColor) {
				b.WriteString(colors.LightBlue.String())
				b.WriteString("    Sources:")
				b.WriteString(colors.Reset.String())
				b.WriteString("\n")
			} else {
				b.WriteString("    Sources:\n")
			}
			for _, src := range f.Sources {
				b.WriteString("      - ")
				if bool(useColor) {
					b.WriteString(colors.LightGray.String())
					b.WriteString(string(src))
					b.WriteString(colors.Reset.String())
				} else {
					b.WriteString(string(src))
				}
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("\n")
	_, err := fmt.Fprint(w, b.String())
	return err
}
