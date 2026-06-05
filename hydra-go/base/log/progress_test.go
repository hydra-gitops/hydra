package log

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDummyProgressBars_Write(t *testing.T) {
	var buf bytes.Buffer
	pb := NewDummyProgressBarsTo(&buf)
	n, err := pb.Write([]byte("hello\n"))
	require.NoError(t, err)
	require.Equal(t, 6, n)
	require.Equal(t, "hello\n", buf.String())
	require.NoError(t, pb.FlushBeforeStdout())
	require.NoError(t, pb.Close())
}

func TestNewProgress_nilWhenTerminalProgressUIDisabled(t *testing.T) {
	SetTerminalProgressUI(false)
	t.Cleanup(func() { SetTerminalProgressUI(false) })
	Configure(Config{
		Level:        LevelInfo,
		ProgressBars: NewDummyProgressBars(),
	})
	p, err := Default().NewProgress("op", 3)
	require.NoError(t, err)
	require.Nil(t, p)
}

func TestNewProgress_dummyAdvanceWhenEnabled(t *testing.T) {
	SetTerminalProgressUI(true)
	t.Cleanup(func() {
		SetTerminalProgressUI(false)
		Configure(Config{Level: LevelInfo})
	})
	Configure(Config{
		Level:        LevelInfo,
		ProgressBars: NewDummyProgressBars(),
	})
	p, err := Default().NewProgress("op", 2)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Close())
}

func TestDummyProgress_NewTaskSetDetailAdvance(t *testing.T) {
	SetTerminalProgressUI(true)
	t.Cleanup(func() {
		SetTerminalProgressUI(false)
		Configure(Config{Level: LevelInfo})
	})
	Configure(Config{
		Level:        LevelInfo,
		ProgressBars: NewDummyProgressBars(),
	})
	p, err := Default().NewProgress("op", 3)
	require.NoError(t, err)
	require.NotNil(t, p)
	task := p.NewTask("")
	require.NotNil(t, task)
	task.SetDetail("step-a")
	p.Advance(1, 3)
	task.SetDetail("step-b")
	p.Advance(2, 3)
	require.NoError(t, task.Close())
	require.NoError(t, p.Close())
}

func TestDummyProgress_TaskCloseIdempotent(t *testing.T) {
	SetTerminalProgressUI(true)
	t.Cleanup(func() {
		SetTerminalProgressUI(false)
		Configure(Config{Level: LevelInfo})
	})
	Configure(Config{
		Level:        LevelInfo,
		ProgressBars: NewDummyProgressBars(),
	})
	p, err := Default().NewProgress("op", 1)
	require.NoError(t, err)
	task := p.NewTask("x")
	require.NoError(t, task.Close())
	require.NoError(t, task.Close())
	require.NoError(t, p.Close())
}

func TestFlushProgressForStdout_keepsProgressContainer(t *testing.T) {
	SetTerminalProgressUI(true)
	t.Cleanup(func() {
		SetTerminalProgressUI(false)
		Configure(Config{Level: LevelInfo})
	})
	Configure(Config{
		Level:        LevelInfo,
		ProgressBars: NewDummyProgressBars(),
	})
	p, err := Default().NewProgress("op", 1)
	require.NoError(t, err)
	require.NotNil(t, p)

	FlushProgressForStdout()
	require.NotNil(t, ActiveProgressBars(), "footer container stays installed for later NewProgress")

	p2, err := Default().NewProgress("op2", 1)
	require.NoError(t, err)
	require.NotNil(t, p2)
	require.NoError(t, p2.Close())
}

func TestCloseActiveProgressBars_restoresLoggingWithoutProgressContainer(t *testing.T) {
	SetTerminalProgressUI(true)
	t.Cleanup(func() {
		SetTerminalProgressUI(false)
		Configure(Config{Level: LevelInfo})
	})
	Configure(Config{
		Level:        LevelInfo,
		ProgressBars: NewDummyProgressBars(),
	})
	p, err := Default().NewProgress("op", 1)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Close())

	CloseActiveProgressBars()
	require.Nil(t, ActiveProgressBars())
	p2, err := Default().NewProgress("after-close", 1)
	require.NoError(t, err)
	require.Nil(t, p2, "progress container removed; NewProgress should return nil")
}

func TestDummyProgress_ProgressCloseClearsTasks(t *testing.T) {
	SetTerminalProgressUI(true)
	t.Cleanup(func() {
		SetTerminalProgressUI(false)
		Configure(Config{Level: LevelInfo})
	})
	Configure(Config{
		Level:        LevelInfo,
		ProgressBars: NewDummyProgressBars(),
	})
	p, err := Default().NewProgress("op", 1)
	require.NoError(t, err)
	_ = p.NewTask("a")
	_ = p.NewTask("b")
	require.NoError(t, p.Close())
}
