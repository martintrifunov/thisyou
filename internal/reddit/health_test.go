package reddit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckHealthReportsEverySource(t *testing.T) {
	online := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer online.Close()
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer limited.Close()
	client := NewClient()
	client.ArcticShift = online.URL
	client.PullPush = limited.URL
	health := client.CheckHealth(context.Background())
	if len(health) != 2 || !health[0].Available || health[1].Available || health[1].StatusCode != http.StatusTooManyRequests {
		t.Fatalf("health = %#v", health)
	}
}
