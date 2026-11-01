package api

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A bigfile upload may be slow but not stalled: the deadline is an idle one that
// every read moves, inside a total of the stall allowance plus the time the
// slowest promised rate needs (E1).
func bigUpload(t *testing.T, s *Server, token string, declared int, chunks int, pause time.Duration) (int, time.Duration) {
	t.Helper()
	return bigUploadTo(t, s, token, "name=slow&memory=true", declared, chunks, pause)
}

func bigUploadTo(t *testing.T, s *Server, token, query string, declared int, chunks int, pause time.Duration) (int, time.Duration) {
	t.Helper()
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	c, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	fmt.Fprintf(c, "PUT /bigfile?record=%%23svc@h&%s HTTP/1.1\r\nHost: h\r\n%s: %s\r\nContent-Length: %d\r\n\r\n", query, HeaderToken, token, declared)
	start := time.Now()
	per := declared / max(chunks, 1)
	for i := 0; i < chunks; i++ {
		if _, err := c.Write([]byte(strings.Repeat("x", per))); err != nil {
			break // the daemon has already answered and closed
		}
		time.Sleep(pause)
	}
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		t.Fatalf("no answer to the upload: %v", err)
	}
	return resp.StatusCode, time.Since(start)
}

func shrinkBigPutClock(t *testing.T, idle time.Duration, rate int64) {
	was, rateWas := bodyWait, bigPutRate
	bodyWait, bigPutRate = idle, rate
	t.Cleanup(func() { bodyWait, bigPutRate = was, rateWas })
}

func TestBigPutSlowButSteadyUploadFinishes(t *testing.T) {
	s, tok := bigServer(t)
	shrinkBigPutClock(t, 300*time.Millisecond, 50) // 100 bytes: 300 ms idle, 2.3 s in all
	// Ten chunks 100 ms apart take a second, past the 300 ms a fixed deadline gave.
	code, took := bigUpload(t, s, tok("alice@h"), 100, 10, 100*time.Millisecond)
	if code != http.StatusOK {
		t.Fatalf("a steady upload slower than the old fixed deadline = %d, want 200 (took %v)", code, took)
	}
	if took < 900*time.Millisecond {
		t.Fatalf("the upload took %v: it did not outlast the 300 ms a fixed deadline would give", took)
	}
}

func TestBigPutStalledUploadIsAnswered408(t *testing.T) {
	s, tok := bigServer(t)
	shrinkBigPutClock(t, 300*time.Millisecond, 50)
	// Half the body, then silence for longer than the idle allowance.
	code, took := bigUpload(t, s, tok("alice@h"), 100, 2, 800*time.Millisecond)
	if code != http.StatusRequestTimeout {
		t.Fatalf("a stalled upload = %d after %v, want 408", code, took)
	}
}

func TestBigPutTrickleBeyondTheBudgetIsAnswered408(t *testing.T) {
	s, tok := bigServer(t)
	shrinkBigPutClock(t, 300*time.Millisecond, 1000) // 100 bytes: 300 ms idle, 0.4 s in all
	// A byte every 50 ms never idles for 300 ms, but would need 5 s.
	code, took := bigUpload(t, s, tok("alice@h"), 100, 100, 50*time.Millisecond)
	if code != http.StatusRequestTimeout {
		t.Fatalf("a trickle past the total budget = %d after %v, want 408", code, took)
	}
	if took > 2*time.Second {
		t.Fatalf("the trickle was cut after %v, want about 0.4 s", took)
	}
}

func TestBigPutBudgetIsTheIdleAllowanceAndTheRateTime(t *testing.T) {
	shrinkBigPutClock(t, 30*time.Second, 64<<10)
	for _, c := range []struct {
		length int64
		want   time.Duration
	}{{0, 30 * time.Second}, {64 << 10, 31 * time.Second}, {32 << 20, 542 * time.Second}, {-1, 542 * time.Second}, {1 << 40, 542 * time.Second}} {
		if got := bigPutBudget(c.length); got != c.want {
			t.Errorf("bigPutBudget(%d) = %v, want %v", c.length, got, c.want)
		}
	}
}

// An early refusal drains what the client declared for bigDrain at most, and a client that
// dribbles its bytes does not stretch that to the upload's idle allowance or budget.
func TestBigPutEarlyRefusalDrainStaysShortForADribblingClient(t *testing.T) {
	s, tok := bigServer(t)
	shrinkBigPutClock(t, 300*time.Millisecond, 1000) // the upload budget of 1000 bytes is 1.3 s
	was := bigDrain
	bigDrain = 300 * time.Millisecond
	t.Cleanup(func() { bigDrain = was })
	// ttl=0 is refused before the body is read; a byte every 100 ms keeps each read inside the idle allowance.
	code, took := bigUploadTo(t, s, tok("alice@h"), "name=k&ttl=0", 1000, 20, 100*time.Millisecond)
	if code != http.StatusBadRequest {
		t.Fatalf("an early refusal = %d, want 400", code)
	}
	if took > 900*time.Millisecond {
		t.Fatalf("the drain of a dribbling client lasted %v, want about the 300 ms drain bound", took)
	}
}
