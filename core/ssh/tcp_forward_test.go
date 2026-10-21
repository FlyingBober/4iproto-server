package sshserver

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"Abdal_4iProto_Server/core/config"
	"Abdal_4iProto_Server/core/security"
	"Abdal_4iProto_Server/core/traffic"
	"golang.org/x/crypto/ssh"
)

type mockNewChannel struct {
	channelType string
	extraData   []byte
	channel     ssh.Channel
	reqs        <-chan *ssh.Request
}

func (m *mockNewChannel) ChannelType() string {
	return m.channelType
}

func (m *mockNewChannel) ExtraData() []byte {
	return m.extraData
}

func (m *mockNewChannel) Accept() (ssh.Channel, <-chan *ssh.Request, error) {
	return m.channel, m.reqs, nil
}

func (m *mockNewChannel) Reject(reason ssh.RejectionReason, message string) error {
	return nil
}

type mockChannel struct {
	r io.Reader
	w io.WriteCloser
}

func (m *mockChannel) Read(data []byte) (int, error) {
	return m.r.Read(data)
}

func (m *mockChannel) Write(data []byte) (int, error) {
	return m.w.Write(data)
}

func (m *mockChannel) Close() error {
	return m.w.Close()
}

func (m *mockChannel) CloseWrite() error {
	return m.w.Close()
}

func (m *mockChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	return false, nil
}

func (m *mockChannel) Stderr() io.ReadWriter {
	return nil
}

func TestHandleDirectTCPIP_CompletesCleanly(t *testing.T) {
	// Start a local echo server as target
	targetListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on target: %v", err)
	}
	defer targetListener.Close()

	targetPort := targetListener.Addr().(*net.TCPAddr).Port

	var targetWg sync.WaitGroup
	targetWg.Add(1)
	go func() {
		defer targetWg.Done()
		conn, err := targetListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				_, _ = conn.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Build direct-tcpip request payload
	// string HostToConnect, uint32 PortToConnect, string OriginatorAddress, uint32 OriginatorPort
	type directTCPIPReq struct {
		HostToConnect     string
		PortToConnect     uint32
		OriginatorAddress string
		OriginatorPort    uint32
	}
	reqData := ssh.Marshal(directTCPIPReq{
		HostToConnect:     "127.0.0.1",
		PortToConnect:     uint32(targetPort),
		OriginatorAddress: "127.0.0.1",
		OriginatorPort:    12345,
	})

	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()

	serverChan := &mockChannel{
		r: serverR,
		w: serverW,
	}

	newChan := &mockNewChannel{
		channelType: "direct-tcpip",
		extraData:   reqData,
		channel:     serverChan,
		reqs:        make(chan *ssh.Request),
	}

	config.SetUsers(map[string]config.User{
		"testuser": {
			Username:   "testuser",
			MaxTotalMB: 0,
		},
	})

	handler := &Handler{
		Traffic: traffic.NewStore(),
		Guard:   security.NewGuard(),
	}

	done := make(chan struct{})
	go func() {
		handler.HandleDirectTCPIP(newChan, "testuser", "127.0.0.1")
		close(done)
	}()

	// Send some data from client
	testMsg := []byte("hello 4iproto")
	go func() {
		_, _ = clientW.Write(testMsg)
		// Close client write after sending
		_ = clientW.Close()
	}()

	// Read reply on client
	reply := make([]byte, len(testMsg))
	_, _ = io.ReadFull(clientR, reply)
	_ = clientR.Close()

	// Verify that HandleDirectTCPIP actually returns within 3 seconds!
	// In the original buggy code, it would hang forever!
	select {
	case <-done:
		t.Log("HandleDirectTCPIP returned successfully without hanging!")
	case <-time.After(3 * time.Second):
		t.Fatal("TIMED OUT: HandleDirectTCPIP is hanging (goroutine leak / deadlock)!")
	}
}
