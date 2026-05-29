// debug_esxi tests VMRC and WebMKS connections to ESXi.
// Usage:
//   go run ./cmd/debug_esxi vmrc <host> <ticket>   — test VMRC port 902 handshake
//   go run ./cmd/debug_esxi ws   <wss-url>         — test WebSocket connection
package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./cmd/debug_esxi <vmrc|ws> [args...]")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "vmrc":
		if len(os.Args) < 4 {
			fmt.Println("Usage: go run ./cmd/debug_esxi vmrc <host> <ticket>")
			os.Exit(1)
		}
		testVMRC(os.Args[2], os.Args[3])
	case "ws":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run ./cmd/debug_esxi ws <wss-url>")
			os.Exit(1)
		}
		testWS(os.Args[2])
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func testVMRC(host, ticket string) {
	addr := host + ":902"
	fmt.Printf("[1] Plain TCP connect to %s\n", addr)

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		fmt.Printf("    FAILED: %v\n", err)
		os.Exit(1)
	}
	r := bufio.NewReader(conn)

	// Read banner on plain TCP (no deadline yet — banner comes immediately)
	conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	banner, _ := r.ReadString('\n')
	fmt.Printf("[2] Banner (plain TCP): %s\n", strings.TrimRight(banner, "\r\n"))

	// Banner says "SSL Required" — upgrade to TLS immediately before any commands
	fmt.Println("[3] Upgrading to TLS (SSL Required)...")
	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
		ServerName:         host,
	})
	tlsConn.SetDeadline(time.Now().Add(15 * time.Second)) //nolint:errcheck
	if err := tlsConn.Handshake(); err != nil {
		fmt.Printf("    TLS FAILED: %v\n", err)
		conn.Close()
		os.Exit(1)
	}
	fmt.Println("    TLS OK")

	r2 := bufio.NewReader(tlsConn)

	// USER
	fmt.Fprintf(tlsConn, "USER %s\r\n", ticket)
	resp, _ := r2.ReadString('\n')
	fmt.Printf("[4] USER resp: %s\n", strings.TrimRight(resp, "\r\n"))

	// PASS
	fmt.Fprintf(tlsConn, "PASS %s\r\n", ticket)
	resp, _ = r2.ReadString('\n')
	fmt.Printf("[5] PASS resp: %s\n", strings.TrimRight(resp, "\r\n"))

	if !strings.HasPrefix(resp, "230") {
		fmt.Println("    Auth failed")
		tlsConn.Close()
		os.Exit(1)
	}

	// CONNECT — the VMRC CONNECT command needs the VM config file path (CfgFile),
	// NOT the ticket UUID. The ticket is only for USER/PASS authentication.
	// CfgFile format: "[datastore] path/to/vm.vmx"
	// Try ticket first (some ESXi versions accept it), then cfgFile if provided.
	connectCmd := fmt.Sprintf("CONNECT %s mks\r\n", ticket)
	fmt.Printf("[6] Sending CONNECT with ticket: %q\n", connectCmd)
	tlsConn.Write([]byte(connectCmd)) //nolint:errcheck
	resp, err = r2.ReadString('\n')
	fmt.Printf("    resp: %q err=%v\n", strings.TrimRight(resp, "\r\n"), err)

	if strings.HasPrefix(resp, "200") {
		fmt.Println("    SUCCESS with ticket!")
		buf := make([]byte, 32)
		n, _ := r2.Read(buf)
		fmt.Printf("    First VNC bytes: %q\n", string(buf[:n]))
		tlsConn.Close()
		return
	}

	// Try with cfgFile (4th arg)
	if len(os.Args) >= 5 {
		cfgFile := os.Args[4]
		connectCmd2 := fmt.Sprintf("CONNECT %s mks\r\n", cfgFile)
		fmt.Printf("[7] Sending CONNECT with cfgFile: %q\n", connectCmd2)
		tlsConn.Write([]byte(connectCmd2)) //nolint:errcheck
		resp, err = r2.ReadString('\n')
		fmt.Printf("    resp: %q err=%v\n", strings.TrimRight(resp, "\r\n"), err)
		if strings.HasPrefix(resp, "200") {
			fmt.Println("    SUCCESS with cfgFile!")
			buf := make([]byte, 32)
			n, _ := r2.Read(buf)
			fmt.Printf("    First VNC bytes: %q\n", string(buf[:n]))
		}
	} else {
		fmt.Println("    Hint: pass the VM cfgFile as 4th arg: go run ./cmd/debug_esxi vmrc <host> <ticket> '<cfgFile>'")
		fmt.Println("    CfgFile looks like: [datastore1] vm/vm.vmx")
		fmt.Println("    Check the backend logs for 'cfg_file' after requesting a console session")
	}
}

func testWS(wsURL string) {
	u, _ := url.Parse(wsURL)
	host := u.Hostname()
	fmt.Printf("Testing WS: %s (host=%s)\n", wsURL, host)

	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true, ServerName: host}, //nolint:gosec
		HandshakeTimeout: 10 * time.Second,
		Subprotocols:     []string{"vmware-wmks"},
	}
	conn, resp, err := dialer.Dial(wsURL, http.Header{"Origin": []string{"https://" + host}})
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		if resp != nil {
			fmt.Printf("HTTP %d\n", resp.StatusCode)
		}
		os.Exit(1)
	}
	fmt.Printf("SUCCESS subprotocol=%q\n", conn.Subprotocol())
	conn.Close()
}
