package infisicalsession

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginServerShutdownDeliversPendingResponse(t *testing.T) {
	entered, draining := make(chan struct{}), make(chan struct{})
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		// Hold the response until shutdown starts to reproduce a callback that
		// is still writing while the login result is being saved.
		select {
		case <-draining:
		case <-r.Context().Done():
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "login complete")
	}))
	server.Config.RegisterOnShutdown(func() { close(draining) })
	server.Start()
	defer server.Close()

	response := make(chan error, 1)
	client := &http.Client{Timeout: 5 * time.Second}
	go func() {
		resp, err := client.Get(server.URL)
		if err == nil {
			defer resp.Body.Close()
			var body []byte
			body, err = io.ReadAll(resp.Body)
			if err == nil && (resp.StatusCode != http.StatusOK || string(body) != "login complete") {
				t.Errorf("incomplete callback response: status=%d body=%q", resp.StatusCode, body)
			}
		}
		response <- err
	}()
	select {
	case <-entered:
	case err := <-response:
		t.Fatalf("callback did not reach the server: %v", err)
	}

	shutdownLoginServer(context.Background(), server.Config)
	if err := <-response; err != nil {
		t.Fatalf("shutdown interrupted the callback response: %v", err)
	}
}

func TestLoginServerShutdownAbortsCancelledRequest(t *testing.T) {
	entered, aborted := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(aborted)
	}))
	defer server.Close()

	response := make(chan error, 1)
	client := &http.Client{Timeout: 5 * time.Second}
	go func() {
		resp, err := client.Get(server.URL)
		if resp != nil {
			resp.Body.Close()
		}
		response <- err
	}()
	select {
	case <-entered:
	case err := <-response:
		t.Fatalf("callback did not reach the server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	shutdownLoginServer(ctx, server.Config)
	if err := <-response; err == nil {
		t.Fatal("cancelled callback unexpectedly succeeded")
	}
	select {
	case <-aborted:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled callback handler was left running")
	}
}
