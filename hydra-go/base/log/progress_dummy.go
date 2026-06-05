package log

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
)

// dummyProgressBars writes log lines to w (typically stderr) and creates non-visual progress steps (debug only).
type dummyProgressBars struct {
	w    io.Writer
	mu   sync.Mutex
	open []*dummyProgress
}

// NewDummyProgressBars returns a ProgressBars that forwards [io.Writer] output to w and implements NewProgress
// without a terminal bar (Advance details at debug level when terminal progress UI is enabled).
func NewDummyProgressBars() ProgressBars {
	return NewDummyProgressBarsTo(os.Stderr)
}

// NewDummyProgressBarsTo is like [NewDummyProgressBars] but uses w as the sink for [io.Writer] writes.
func NewDummyProgressBarsTo(w io.Writer) ProgressBars {
	if w == nil {
		w = os.Stderr
	}
	return &dummyProgressBars{w: w}
}

func (d *dummyProgressBars) Write(p []byte) (int, error) {
	return d.w.Write(p)
}

func (d *dummyProgressBars) Close() error {
	return nil
}

func (d *dummyProgressBars) register(p *dummyProgress) {
	d.mu.Lock()
	d.open = append(d.open, p)
	d.mu.Unlock()
}

func (d *dummyProgressBars) unregister(p *dummyProgress) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, x := range d.open {
		if x == p {
			d.open = append(d.open[:i], d.open[i+1:]...)
			return
		}
	}
}

func (d *dummyProgressBars) FlushBeforeStdout() error {
	d.mu.Lock()
	list := d.open
	d.open = nil
	d.mu.Unlock()
	for _, p := range list {
		_ = p.Close()
	}
	_ = os.Stderr.Sync()
	SyncStdoutBestEffort()
	return nil
}

func (d *dummyProgressBars) NewProgress(operation string, total int) (Progress, error) {
	_ = os.Stderr.Sync()
	if !terminalProgressUI.Load() {
		return nil, nil
	}
	dp := &dummyProgress{footer: d, operation: operation, total: total}
	d.register(dp)
	if terminalProgressUI.Load() {
		Default().DebugLog(Hydra().Child("progress").Child("dummy"), "{operation} bar opened",
			String("operation", operation),
			Int("total", total),
		)
	}
	return dp, nil
}

type dummyProgress struct {
	footer    *dummyProgressBars
	operation string
	total     int
	mu        sync.Mutex
	tasks     []*dummyProgressTask
	closed    atomic.Bool
}

func (d *dummyProgress) Advance(index, total int) {
	if !terminalProgressUI.Load() {
		return
	}
	Default().DebugLog(Hydra().Child("progress").Child("dummy"), "{operation} bar advance",
		String("operation", d.operation),
		Int("index", index),
		Int("total", total),
	)
}

func (d *dummyProgress) NewTask(name string) ProgressTask {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed.Load() {
		return noopDummyTask{}
	}
	t := &dummyProgressTask{parent: d}
	d.tasks = append(d.tasks, t)
	if terminalProgressUI.Load() {
		Default().DebugLog(Hydra().Child("progress").Child("dummy"), "{operation} status opened",
			String("operation", d.operation),
			String("name", name),
		)
	}
	return t
}

func (d *dummyProgress) Close() error {
	if !d.closed.CompareAndSwap(false, true) {
		return nil
	}
	d.mu.Lock()
	for _, t := range d.tasks {
		t.closed.Store(true)
	}
	d.tasks = d.tasks[:0]
	d.mu.Unlock()
	if terminalProgressUI.Load() {
		Default().DebugLog(Hydra().Child("progress").Child("dummy"), "{operation} bar closed",
			String("operation", d.operation),
		)
	}
	if d.footer != nil {
		d.footer.unregister(d)
		d.footer = nil
	}
	return nil
}

type dummyProgressTask struct {
	parent *dummyProgress
	closed atomic.Bool
}

func (t *dummyProgressTask) SetDetail(detail string) {
	if t.closed.Load() {
		return
	}
	if !terminalProgressUI.Load() {
		return
	}
	Default().DebugLog(Hydra().Child("progress").Child("dummy"), "{operation} status updated",
		String("operation", t.parent.operation),
		String("detail", detail),
	)
}

func (t *dummyProgressTask) Close() error {
	if t.parent == nil {
		return nil
	}
	if !t.closed.CompareAndSwap(false, true) {
		return nil
	}
	t.parent.mu.Lock()
	out := t.parent.tasks[:0]
	for _, x := range t.parent.tasks {
		if x != t {
			out = append(out, x)
		}
	}
	t.parent.tasks = out
	t.parent.mu.Unlock()
	if terminalProgressUI.Load() && t.parent != nil {
		Default().DebugLog(Hydra().Child("progress").Child("dummy"), "{operation} status closed",
			String("operation", t.parent.operation),
		)
	}
	return nil
}

type noopDummyTask struct{}

func (noopDummyTask) SetDetail(string) {}
func (noopDummyTask) Close() error     { return nil }
