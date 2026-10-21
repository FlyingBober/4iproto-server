/*
 **********************************************************************
 * -------------------------------------------------------------------
 * Project Name : Abdal 4iProto Server
 * File Name    : ratelimit.go
 * Programmer   : Ebrahim Shafiei (EbraSha)
 * Email        : Prof.Shafiei@Gmail.com
 * Created On   : 2026-06-30 04:12:29
 * Description  : Per-user rate limiting with throttled io.Reader and io.Writer wrappers
 * -------------------------------------------------------------------
 *
 * "Coding is an engaging and beloved hobby for me. I passionately and insatiably pursue knowledge in cybersecurity and programming."
 * – Ebrahim Shafiei
 *
 **********************************************************************
 */

package traffic

import (
	"context"
	"io"
	"log"

	"golang.org/x/time/rate"
)

// InitRateLimiter initializes a rate limiter for a user.
func (s *Store) InitRateLimiter(username string, maxSpeedKBPS int) {
	if maxSpeedKBPS <= 0 {
		return
	}

	bytesPerSecond := rate.Limit(maxSpeedKBPS * 1024)
	burst := maxSpeedKBPS * 1024
	if burst > 1024*1024 {
		burst = 1024 * 1024
	}

	limiter := rate.NewLimiter(bytesPerSecond, burst)
	s.rateLimiters.Store(username, limiter)
	log.Printf("⚡ Rate limiter initialized for %s: %d KB/s (burst: %d bytes)", username, maxSpeedKBPS, burst)
}

// GetRateLimiter returns the rate limiter for a user (nil if unlimited).
func (s *Store) GetRateLimiter(username string) *rate.Limiter {
	if limiter, ok := s.rateLimiters.Load(username); ok {
		return limiter.(*rate.Limiter)
	}
	return nil
}

// ThrottledReader wraps an io.Reader with rate limiting.
type ThrottledReader struct {
	R       io.Reader
	Limiter *rate.Limiter
	Ctx     context.Context
}

func (tr *ThrottledReader) Read(p []byte) (n int, err error) {
	if tr.Limiter == nil {
		return tr.R.Read(p)
	}
	n, err = tr.R.Read(p)
	if n <= 0 {
		return n, err
	}
	ctx := tr.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	burst := tr.Limiter.Burst()
	if burst <= 0 || n <= burst {
		err = tr.Limiter.WaitN(ctx, n)
		return n, err
	}
	// If n > burst, wait in chunks to avoid rate: Wait(n) exceeds limiter's budget
	rem := n
	for rem > 0 {
		chunk := rem
		if chunk > burst {
			chunk = burst
		}
		if err = tr.Limiter.WaitN(ctx, chunk); err != nil {
			return n, err
		}
		rem -= chunk
	}
	return n, nil
}

// ThrottledWriter wraps an io.Writer with rate limiting.
type ThrottledWriter struct {
	W       io.Writer
	Limiter *rate.Limiter
	Ctx     context.Context
}

func (tw *ThrottledWriter) Write(p []byte) (n int, err error) {
	if tw.Limiter == nil {
		return tw.W.Write(p)
	}
	ctx := tw.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	burst := tw.Limiter.Burst()
	if burst <= 0 {
		return tw.W.Write(p)
	}

	totalWritten := 0
	for len(p) > 0 {
		chunk := len(p)
		if chunk > burst {
			chunk = burst
		}
		if err := tw.Limiter.WaitN(ctx, chunk); err != nil {
			return totalWritten, err
		}
		nw, err := tw.W.Write(p[:chunk])
		totalWritten += nw
		if err != nil {
			return totalWritten, err
		}
		p = p[nw:]
	}
	return totalWritten, nil
}
