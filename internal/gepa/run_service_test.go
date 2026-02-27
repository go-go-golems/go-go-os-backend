package gepa

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInMemoryRunService_StateTransitionsToCompleted(t *testing.T) {
	service := NewInMemoryRunService(60*time.Millisecond, 2*time.Second, 2)

	record, err := service.Start(context.Background(), ScriptDescriptor{
		ID:   "script-a",
		Name: "script-a",
		Path: "/tmp/script-a.js",
	}, StartRunRequest{
		ScriptID: "script-a",
	})
	require.NoError(t, err)
	require.Equal(t, RunStatusRunning, record.Status)

	time.Sleep(120 * time.Millisecond)

	got, found, err := service.Get(context.Background(), record.RunID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, RunStatusCompleted, got.Status)

	events, found, err := service.Events(context.Background(), record.RunID, 0)
	require.NoError(t, err)
	require.True(t, found)
	require.GreaterOrEqual(t, len(events), 2)
	require.Equal(t, "run.started", events[0].Type)
	require.Equal(t, "run.completed", events[len(events)-1].Type)
}

func TestInMemoryRunService_CancelRaceKeepsSingleTerminalEvent(t *testing.T) {
	service := NewInMemoryRunService(400*time.Millisecond, 5*time.Second, 4)

	record, err := service.Start(context.Background(), ScriptDescriptor{
		ID:   "script-race",
		Name: "script-race",
		Path: "/tmp/script-race.js",
	}, StartRunRequest{ScriptID: "script-race"})
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _, _ = service.Cancel(context.Background(), record.RunID)
	}()
	go func() {
		defer wg.Done()
		_, _, _ = service.Cancel(context.Background(), record.RunID)
	}()
	wg.Wait()

	time.Sleep(60 * time.Millisecond)

	got, found, err := service.Get(context.Background(), record.RunID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, RunStatusCanceled, got.Status)

	events, found, err := service.Events(context.Background(), record.RunID, 0)
	require.NoError(t, err)
	require.True(t, found)

	canceledCount := 0
	completedCount := 0
	failedCount := 0
	for _, event := range events {
		switch event.Type {
		case "run.canceled":
			canceledCount++
		case "run.completed":
			completedCount++
		case "run.failed":
			failedCount++
		}
	}
	require.Equal(t, 1, canceledCount, "run.canceled should only be emitted once")
	require.Equal(t, 0, completedCount, "canceled run should not also complete")
	require.Equal(t, 0, failedCount, "canceled run should not be marked failed")
}

func TestInMemoryRunService_EventsAfterSeqReplay(t *testing.T) {
	service := NewInMemoryRunService(70*time.Millisecond, 3*time.Second, 2)

	record, err := service.Start(context.Background(), ScriptDescriptor{
		ID:   "script-replay",
		Name: "script-replay",
		Path: "/tmp/script-replay.js",
	}, StartRunRequest{ScriptID: "script-replay"})
	require.NoError(t, err)

	time.Sleep(140 * time.Millisecond)

	all, found, err := service.Events(context.Background(), record.RunID, 0)
	require.NoError(t, err)
	require.True(t, found)
	require.GreaterOrEqual(t, len(all), 2)

	afterFirst, found, err := service.Events(context.Background(), record.RunID, all[0].Seq)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, afterFirst, len(all)-1)
	for _, ev := range afterFirst {
		require.Greater(t, ev.Seq, all[0].Seq)
	}
}
