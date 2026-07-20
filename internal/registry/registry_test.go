package registry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchRemoteHonorsClientTimeout(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 10 * time.Millisecond}
	_, err := fetchRemote(server.URL, client)
	<-started
	close(release)

	if err == nil {
		t.Fatal("fetchRemote returned no error after the client timeout")
	}
	if !strings.Contains(err.Error(), "fetch registry") {
		t.Fatalf("fetchRemote returned an unexpected error: %v", err)
	}
}
