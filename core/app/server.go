/*
 **********************************************************************
 * -------------------------------------------------------------------
 * Project Name : Abdal 4iProto Server
 * File Name    : server.go
 * Programmer   : Ebrahim Shafiei (EbraSha)
 * Email        : Prof.Shafiei@Gmail.com
 * Created On   : 2026-06-30 04:12:29
 * Description  : Application bootstrap and SSH listener orchestration
 * -------------------------------------------------------------------
 *
 * "Coding is an engaging and beloved hobby for me. I passionately and insatiably pursue knowledge in cybersecurity and programming."
 * – Ebrahim Shafiei
 *
 **********************************************************************
 */

package app

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"Abdal_4iProto_Server/core/config"
	"Abdal_4iProto_Server/core/dnstt"
	"Abdal_4iProto_Server/core/security"
	"Abdal_4iProto_Server/core/session"
	sshserver "Abdal_4iProto_Server/core/ssh"
	"Abdal_4iProto_Server/core/traffic"
)

// Run starts the Abdal 4iProto Server (blocking).
func Run() {
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(250 * 1024 * 1024) // 250 MiB soft memory limit
	}

	trafficStore := traffic.NewStore()
	guard := security.NewGuard()

	config.LoadUsers(func(username string, maxSpeedKBPS int) {
		trafficStore.InitRateLimiter(username, maxSpeedKBPS)
	})
	config.LoadServerConfig()
	guard.LoadBlockedIPs()
	trafficStore.LoadExistingFiles()

	sm := session.GetManager()
	sm.StartCleanupRoutine()

	handler := &sshserver.Handler{
		Sessions: sm,
		Traffic:  trafficStore,
		Guard:    guard,
	}
	sshConfig := handler.BuildServerConfig()

	serverCfg := config.Server()
	if serverCfg.DNSTTEnabled {
		go dnstt.StartGateway(serverCfg, guard)
	}

	var listeners []net.Listener
	var listenersMu sync.Mutex

	for _, port := range serverCfg.Ports {
		listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			log.Printf("❌ Failed to listen on port %d: %v", port, err)
			continue
		}
		listenersMu.Lock()
		listeners = append(listeners, listener)
		listenersMu.Unlock()

		log.Printf("🚀 Abdal 4iProto Server listening on port %d", port)

		go func(l net.Listener, p int) {
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				if tcpConn, ok := conn.(*net.TCPConn); ok {
					tcpConn.SetNoDelay(true)
					tcpConn.SetKeepAlive(true)
					tcpConn.SetKeepAlivePeriod(30 * time.Second)
				}
				go handler.HandleConnection(conn, sshConfig)
			}
		}(listener, port)
	}

	trafficStore.StartAutoSave()
	trafficStore.StartDebugTicker()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	log.Printf("🛑 Received shutdown signal (%v), initiating graceful shutdown...", sig)

	listenersMu.Lock()
	for _, l := range listeners {
		_ = l.Close()
	}
	listenersMu.Unlock()

	log.Printf("💾 Saving all pending user traffic stats...")
	trafficStore.SaveAllTraffic()

	log.Printf("🔒 Closing active sessions and database...")
	if err := sm.Close(); err != nil {
		log.Printf("⚠️ Error closing session database: %v", err)
	}

	log.Printf("👋 Abdal 4iProto Server shutdown cleanly.")
}
