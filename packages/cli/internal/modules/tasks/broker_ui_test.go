package tasks

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestBrokerDisplaySnapshotKeepsStartAcrossSteps(t *testing.T) {
	b := &invocationBroker{token: "test", specs: map[string]leafSpec{"task-0": {Task: Task{Name: "//:silent"}, Environment: []string{"SECRET=value"}}}, events: map[string]leafEvent{}}
	report := func(status string) {
		req := httptest.NewRequest("POST", "/task-0", strings.NewReader(`{"status":"`+status+`"}`))
		req.Header.Set("Authorization", "Bearer test")
		response := httptest.NewRecorder()
		b.ServeHTTP(response, req)
		if response.Code != 204 {
			t.Error(response.Code)
		}
	}
	report("running")
	first := b.snapshot()[0]
	report("running")
	next := b.snapshot()[0]
	if first.StartedAt.IsZero() || first.StartedAt != next.StartedAt {
		t.Fatal("multi-step or duplicate start reset ordering")
	}
	snapshot := b.snapshot()
	snapshot[0].Status = "failed"
	if b.snapshot()[0].Status != "running" {
		t.Fatal("snapshot mutated broker")
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 100 {
			report("running")
		}
	})
	wg.Go(func() {
		for range 100 {
			_ = b.snapshot()
		}
	})
	wg.Wait()
	report("succeeded")
	last := b.snapshot()[0]
	if last.EndedAt.IsZero() || last.StartedAt != first.StartedAt {
		t.Fatal("completion lost lifecycle time")
	}
	raw, _ := json.Marshal(last)
	if strings.Contains(string(raw), "SECRET") || strings.Contains(string(raw), "value") {
		t.Fatal("UI state exposed environment")
	}
}
