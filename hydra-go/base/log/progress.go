package log

import (
	"io"
	"os"
	"sync/atomic"
)

// ProgressBars is the terminal footer container: slog output is written through it as [io.Writer].
// The mpb-backed implementation lives only in cli/progress; [NewDummyProgressBars] is the no-mpb variant.
type ProgressBars interface {
	io.Writer
	NewProgress(operation string, total int) (Progress, error)
	Close() error
	// FlushBeforeStdout aborts any active footer bars, flushes stderr and stdout, and keeps the container
	// so new bars can be created afterward.
	FlushBeforeStdout() error
}

// ProgressTask is one status line rendered below the main progress bar. Use [Progress.NewTask] to create it.
// [ProgressTask.SetDetail] sets the full line text (for example a resource id). [ProgressTask.Close] removes
// only this line and does not change the bar position. Calling Close more than once is a no-op.
type ProgressTask interface {
	SetDetail(detail string)
	Close() error
}

// Progress is one bar. The bar starts when [Logger.NewProgress] returns; call [Progress.Close] to remove it
// and to close every [ProgressTask] created from this Progress instance.
// Log any result line after Close with [Logger.InfoLog] / slog, not as a Close argument.
type Progress interface {
	Advance(index, total int)
	Close() error
	NewTask(name string) ProgressTask
}

var (
	activeProgressBars ProgressBars
	terminalProgressUI atomic.Bool
	noProgressFlag     atomic.Bool
	stdoutTTYAtCliInit atomic.Bool // whether os.Stdout was a TTY before optional mpb stdout pipe install
)

// SetStdoutTTYAtCliInit records whether stdout was a terminal at the start of CLI logging setup, before
// [cli/progress.MpbProgressBars.InstallStdoutProxyIfNeeded] may replace os.Stdout with a pipe.
// Used for human-output color auto-detection when [os.Stdout] is no longer a TTY.
func SetStdoutTTYAtCliInit(v bool) {
	stdoutTTYAtCliInit.Store(v)
}

// StdoutTTYAtCliInit reports the value last passed to [SetStdoutTTYAtCliInit].
func StdoutTTYAtCliInit() bool {
	return stdoutTTYAtCliInit.Load()
}

// SetNoProgress records the global --no-progress flag (dummy footer instead of mpb).
func SetNoProgress(v bool) {
	noProgressFlag.Store(v)
}

// NoProgress reports whether --no-progress was set.
func NoProgress() bool {
	return noProgressFlag.Load()
}

// ActiveProgressBars returns the ProgressBars last passed to [Configure], or nil.
func ActiveProgressBars() ProgressBars {
	return activeProgressBars
}

// SetTerminalProgressUI controls whether [Logger.NewProgress] may create a bar. When false, NewProgress returns (nil, nil).
func SetTerminalProgressUI(enabled bool) {
	terminalProgressUI.Store(enabled)
}

// TerminalProgressUI reports whether terminal footer bars may be shown (TTY, color, and command allow it).
func TerminalProgressUI() bool {
	return terminalProgressUI.Load()
}

// CloseActiveProgressBars shuts down the active terminal progress container (if any) and re-applies the last
// [Configure] settings with ProgressBars cleared so subsequent slog output goes to stderr only.
// Call this before writing command results to stdout to avoid interleaving with mpb redraws.
// It is a no-op when no progress container is active.
func CloseActiveProgressBars() {
	pb := activeProgressBars
	activeProgressBars = nil
	if pb == nil {
		return
	}
	_ = pb.Close()
	cfg := lastLogConfig
	cfg.ProgressBars = nil
	Configure(cfg)
}

// SyncStdoutBestEffort attempts to flush stdout via [os.Stdout.Sync]. It ignores errors because Sync is
// unsupported for some stdout sinks (for example certain character devices or test doubles).
func SyncStdoutBestEffort() {
	_ = os.Stdout.Sync()
}

// FlushProgressForStdout aborts active footer bars (if any), flushes stderr and stdout, and leaves the
// progress container configured so [Logger.NewProgress] can open a new bar later in the same command.
func FlushProgressForStdout() {
	pb := activeProgressBars
	if pb == nil {
		_ = os.Stderr.Sync()
		SyncStdoutBestEffort()
		return
	}
	_ = pb.FlushBeforeStdout()
}
