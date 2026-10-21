/*
 **********************************************************************
 * -------------------------------------------------------------------
 * Project Name : Abdal 4iProto Server
 * File Name    : tcp_forward.go
 * Programmer   : Ebrahim Shafiei (EbraSha)
 * Email        : Prof.Shafiei@Gmail.com
 * Created On   : 2026-06-30 04:12:29
 * Description  : SSH direct-tcpip channel forwarding with traffic accounting
 * -------------------------------------------------------------------
 *
 * "Coding is an engaging and beloved hobby for me. I passionately and insatiably pursue knowledge in cybersecurity and programming."
 * – Ebrahim Shafiei
 *
 **********************************************************************
 */

package sshserver

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"Abdal_4iProto_Server/core/config"
	"Abdal_4iProto_Server/core/logging"
	"Abdal_4iProto_Server/core/security"
	"Abdal_4iProto_Server/core/traffic"

	"golang.org/x/crypto/ssh"
)

const tcpBufferSize = 32 * 1024

var tcpBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, tcpBufferSize)
		return &b
	},
}

// HandleDirectTCPIP forwards a direct-tcpip SSH channel to a remote TCP target.
func (h *Handler) HandleDirectTCPIP(newChannel ssh.NewChannel, username, userIP string) {
	type directTCPIPReq struct {
		HostToConnect     string
		PortToConnect     uint32
		OriginatorAddress string
		OriginatorPort    uint32
	}

	var req directTCPIPReq
	if err := ssh.Unmarshal(newChannel.ExtraData(), &req); err != nil {
		newChannel.Reject(ssh.Prohibited, "could not parse direct-tcpip request")
		return
	}

	target := fmt.Sprintf("%s:%d", req.HostToConnect, req.PortToConnect)

	if security.IsDomainOrIPBlocked(username, req.HostToConnect) {
		log.Printf("🚫 User %s tried to access blocked target: %s", username, req.HostToConnect)
		logging.BlockedAccess(username, req.HostToConnect, userIP)
		newChannel.Reject(ssh.Prohibited, "access to this domain/IP is blocked")
		return
	}

	logging.UserAccess(username, req.HostToConnect, userIP)

	destConn, err := net.DialTimeout("tcp", target, 15*time.Second)
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, "could not connect to target")
		return
	}
	defer destConn.Close()

	if tcpConn, ok := destConn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	channel, requests, err := newChannel.Accept()
	if err != nil {
		return
	}
	defer channel.Close()
	go ssh.DiscardRequests(requests)

	user, _ := config.GetUser(username)
	maxTotalMB := user.MaxTotalMB
	userLimiter := h.Traffic.GetRateLimiter(username)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var downloadWriter io.Writer = channel
	var uploadWriter io.Writer = destConn

	if userLimiter != nil {
		downloadWriter = &traffic.ThrottledWriter{W: channel, Limiter: userLimiter, Ctx: ctx}
		uploadWriter = &traffic.ThrottledWriter{W: destConn, Limiter: userLimiter, Ctx: ctx}
	}

	var bytesSent atomic.Int64
	var bytesReceived atomic.Int64
	var lastSent int64
	var lastReceived int64

	limitExceeded := make(chan struct{}, 1)
	done := make(chan struct{})

	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			cancel()
			_ = channel.Close()
			_ = destConn.Close()
		})
	}

	// Traffic monitoring & rate-limit check goroutine
	var monitorWg sync.WaitGroup
	monitorWg.Add(1)
	go func() {
		defer monitorWg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		const saveEveryNTicks = 5
		tickCount := 0

		for {
			select {
			case <-ticker.C:
				currentSent := bytesSent.Load()
				currentReceived := bytesReceived.Load()
				sentDelta := currentSent - lastSent
				receivedDelta := currentReceived - lastReceived

				if sentDelta > 0 || receivedDelta > 0 {
					if h.Traffic.UpdateRealTime(username, userIP, sentDelta, receivedDelta, maxTotalMB) {
						select {
						case limitExceeded <- struct{}{}:
						default:
						}
						log.Printf("🚫 User %s exceeded traffic limit, closing connection", username)
						closeBoth()
						return
					}
					tickCount++
					if tickCount >= saveEveryNTicks {
						h.Traffic.SaveToFile(username)
						tickCount = 0
					}
					lastSent = currentSent
					lastReceived = currentReceived
				}
			case <-done:
				return
			case <-limitExceeded:
				return
			}
		}
	}()

	var transferWg sync.WaitGroup
	transferWg.Add(2)

	// Goroutine: Target -> Client (Download)
	go func() {
		defer transferWg.Done()
		defer channel.CloseWrite()

		bufPtr := tcpBufPool.Get().(*[]byte)
		defer tcpBufPool.Put(bufPtr)
		buf := *bufPtr

		for {
			select {
			case <-limitExceeded:
				return
			case <-ctx.Done():
				return
			default:
			}

			n, err := destConn.Read(buf)
			if n > 0 {
				bytesReceived.Add(int64(n))
				if _, writeErr := downloadWriter.Write(buf[:n]); writeErr != nil {
					closeBoth()
					break
				}
			}
			if err != nil {
				if err != io.EOF {
					closeBoth()
				}
				break
			}
		}
	}()

	// Goroutine: Client -> Target (Upload)
	go func() {
		defer transferWg.Done()
		defer func() {
			if tc, ok := destConn.(*net.TCPConn); ok {
				_ = tc.CloseWrite()
			}
		}()

		bufPtr := tcpBufPool.Get().(*[]byte)
		defer tcpBufPool.Put(bufPtr)
		buf := *bufPtr

		for {
			select {
			case <-limitExceeded:
				return
			case <-ctx.Done():
				return
			default:
			}

			n, err := channel.Read(buf)
			if n > 0 {
				bytesSent.Add(int64(n))
				if _, writeErr := uploadWriter.Write(buf[:n]); writeErr != nil {
					closeBoth()
					break
				}
			}
			if err != nil {
				if err != io.EOF {
					closeBoth()
				}
				break
			}
		}
	}()

	transferWg.Wait()
	close(done)
	closeBoth()
	monitorWg.Wait()

	sent := bytesSent.Load()
	received := bytesReceived.Load()
	if sent > lastSent || received > lastReceived {
		h.Traffic.UpdateRealTime(username, userIP, sent-lastSent, received-lastReceived, maxTotalMB)
		h.Traffic.SaveToFile(username)
	}
	h.Traffic.UpdateSessionFinalStats(username, sent, received)
}
