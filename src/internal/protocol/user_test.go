package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestUserOmitsUnobservedProviderTimesAndPublishesMeasuredOnes(t *testing.T) {
	body, err := json.Marshal(User{Name: "alice@h"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "github_profile_at") || strings.Contains(string(body), "photo_fetched_at") {
		t.Fatalf("zero provider time was published as a measurement: %s", body)
	}
	at := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	body, err = json.Marshal(User{Name: "alice@h", GithubProfileAt: at, PhotoFetchedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"github_profile_at":"2026-09-17T12:00:00Z"`) || !strings.Contains(string(body), `"photo_fetched_at":"2026-09-17T12:00:00Z"`) {
		t.Fatalf("measured provider times absent: %s", body)
	}
}
