package traffic

import (
	"bytes"
	"context"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestThrottledWriter_Chunking(t *testing.T) {
	var buf bytes.Buffer
	// Limiter with burst = 100 bytes, limit = 1000 bytes/sec
	limiter := rate.NewLimiter(1000, 100)
	tw := &ThrottledWriter{
		W:       &buf,
		Limiter: limiter,
		Ctx:     context.Background(),
	}

	// Payload larger than burst (250 bytes > 100 bytes burst)
	payload := make([]byte, 250)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	start := time.Now()
	n, err := tw.Write(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 250 {
		t.Fatalf("expected 250 bytes written, got %d", n)
	}
	if !bytes.Equal(buf.Bytes(), payload) {
		t.Fatalf("written data does not match payload")
	}
	t.Logf("Write finished in %v", time.Since(start))
}
