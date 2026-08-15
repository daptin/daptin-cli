package mailprobe

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSMTPDeliveryOverImplicitTLS(t *testing.T) {
	tlsConfig, caFile := testTLS(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	message := make(chan string, 1)
	go serveSMTP(t, listener, "sender@mail.test", "smtp-secret", message)

	request := SMTPDeliveryRequest{
		Connection: Connection{Address: listener.Addr().String(), ServerName: "mail.test", CAFile: caFile, TLSMode: TLSImplicit, Timeout: 3 * time.Second},
		Username:   "sender@mail.test", Password: []byte("smtp-secret"), From: "sender@mail.test", To: "receiver@mail.test", CorrelationID: "probe-123",
	}
	result, err := DeliverSMTP(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.TLS.Verified || !result.Authenticated || !result.MessageAccepted {
		t.Fatalf("unexpected result: %#v", result)
	}
	if got := <-message; !strings.Contains(got, "Message-ID: <probe-123@mail.test>") {
		t.Fatalf("message lacks correlation ID: %q", got)
	}
}

func TestSMTPAuthenticationErrorIsSanitized(t *testing.T) {
	tlsConfig, caFile := testTLS(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go serveSMTP(t, listener, "sender@mail.test", "actual-secret", make(chan string, 1))
	_, err = DeliverSMTP(context.Background(), SMTPDeliveryRequest{
		Connection: Connection{Address: listener.Addr().String(), ServerName: "mail.test", CAFile: caFile, TLSMode: TLSImplicit, Timeout: 3 * time.Second},
		Username:   "sender@mail.test", Password: []byte("wrong-secret"), From: "sender@mail.test", To: "receiver@mail.test", CorrelationID: "probe-123",
	})
	if err == nil || err.Error() != "SMTP authentication failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIMAPMessageIDSearchOverImplicitTLS(t *testing.T) {
	tlsConfig, caFile := testTLS(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	commands := make(chan []string, 1)
	go serveIMAP(t, listener, commands)
	result, err := SearchIMAP(context.Background(), IMAPSearchRequest{
		Connection: Connection{Address: listener.Addr().String(), ServerName: "mail.test", CAFile: caFile, TLSMode: TLSImplicit, Timeout: 3 * time.Second},
		Username:   "receiver@mail.test", Password: []byte("imap-secret"), Folder: "INBOX", CorrelationID: "probe-123", PollTimeout: 2 * time.Second, PollInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || !result.FolderSelected || !result.MessageFound {
		t.Fatalf("unexpected result: %#v", result)
	}
	joined := strings.Join(<-commands, "\n")
	if !strings.Contains(joined, `UID SEARCH HEADER Message-ID "<probe-123@mail.test>"`) || strings.Contains(joined, "FETCH") {
		t.Fatalf("unexpected IMAP commands: %s", joined)
	}
}

func TestIMAPFallsBackToBoundedMessageIDHeaderFetch(t *testing.T) {
	tlsConfig, caFile := testTLS(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	commands := make(chan []string, 1)
	go serveIMAPFallback(listener, commands)
	result, err := SearchIMAP(context.Background(), IMAPSearchRequest{
		Connection: Connection{Address: listener.Addr().String(), ServerName: "mail.test", CAFile: caFile, TLSMode: TLSImplicit, Timeout: 3 * time.Second},
		Username:   "receiver@mail.test", Password: []byte("imap-secret"), Folder: "INBOX", CorrelationID: "probe-123", PollTimeout: 2 * time.Second, PollInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.MessageFound {
		t.Fatalf("expected fallback match: %#v", result)
	}
	joined := strings.Join(<-commands, "\n")
	if !strings.Contains(joined, "SEARCH ALL") || !strings.Contains(joined, "BODY.PEEK[HEADER.FIELDS (MESSAGE-ID)]") {
		t.Fatalf("fallback commands missing: %s", joined)
	}
}

func serveSMTP(t *testing.T, listener net.Listener, username, password string, message chan<- string) {
	t.Helper()
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	in := bufio.NewScanner(conn)
	out := bufio.NewWriter(conn)
	write := func(value string) { _, _ = out.WriteString(value + "\r\n"); _ = out.Flush() }
	write("220 test ESMTP")
	in.Scan()
	write("250-test")
	write("250 AUTH LOGIN")
	in.Scan()
	write("334 VXNlcm5hbWU6")
	in.Scan()
	decodedUser, _ := base64.StdEncoding.DecodeString(in.Text())
	write("334 UGFzc3dvcmQ6")
	in.Scan()
	decodedPassword, _ := base64.StdEncoding.DecodeString(in.Text())
	if string(decodedUser) != username || string(decodedPassword) != password {
		write("535 authentication rejected; supplied values omitted")
		return
	}
	write("235 authenticated")
	in.Scan()
	write("250 sender ok")
	in.Scan()
	write("250 recipient ok")
	in.Scan()
	write("354 continue")
	var lines []string
	for in.Scan() {
		if in.Text() == "." {
			break
		}
		lines = append(lines, in.Text())
	}
	message <- strings.Join(lines, "\n")
	write("250 queued")
	in.Scan()
	write("221 bye")
}

func serveIMAP(t *testing.T, listener net.Listener, commands chan<- []string) {
	t.Helper()
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	in := bufio.NewScanner(conn)
	out := bufio.NewWriter(conn)
	write := func(value string) { _, _ = out.WriteString(value + "\r\n"); _ = out.Flush() }
	write("* OK test IMAP ready")
	var seen []string
	for in.Scan() {
		line := in.Text()
		seen = append(seen, line)
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return
		}
		tag := fields[0]
		switch strings.ToUpper(fields[1]) {
		case "LOGIN":
			write(tag + " OK logged in")
		case "SELECT":
			write("* 1 EXISTS")
			write(tag + " OK selected")
		case "UID":
			write("* SEARCH 7")
			write(tag + " OK searched")
		case "LOGOUT":
			write("* BYE")
			write(tag + " OK logout")
			commands <- seen
			return
		default:
			write(tag + " BAD unsupported")
		}
	}
}

func serveIMAPFallback(listener net.Listener, commands chan<- []string) {
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	in := bufio.NewScanner(conn)
	out := bufio.NewWriter(conn)
	write := func(value string) { _, _ = out.WriteString(value + "\r\n"); _ = out.Flush() }
	write("* OK test IMAP ready")
	var seen []string
	for in.Scan() {
		line := in.Text()
		seen = append(seen, line)
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return
		}
		tag := fields[0]
		upper := strings.ToUpper(line)
		switch {
		case strings.Contains(upper, " LOGIN "):
			write(tag + " OK logged in")
		case strings.Contains(upper, " SELECT "):
			write("* 1 EXISTS")
			write(tag + " OK selected")
		case strings.Contains(upper, "UID SEARCH HEADER"):
			write("* SEARCH")
			write(tag + " OK searched")
		case strings.Contains(upper, " SEARCH ALL"):
			write("* SEARCH 1")
			write(tag + " OK searched")
		case strings.Contains(upper, " FETCH "):
			literal := "Message-ID: <probe-123@mail.test>\r\n\r\n"
			_, _ = out.WriteString(fmt.Sprintf("* 1 FETCH (BODY[HEADER.FIELDS (MESSAGE-ID)] {%d}\r\n%s)\r\n%s OK fetched\r\n", len(literal), literal, tag))
			_ = out.Flush()
		case strings.Contains(upper, " LOGOUT"):
			write("* BYE")
			write(tag + " OK logout")
			commands <- seen
			return
		default:
			write(tag + " BAD unsupported")
		}
	}
}

func testTLS(t *testing.T) (*tls.Config, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "mail.test"}, DNSNames: []string{"mail.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}, caFile
}

func ExampleValidateCorrelationID() {
	fmt.Println(ValidateCorrelationID("probe-123"))
	fmt.Println(ValidateCorrelationID("bad@example"))
	// Output:
	// <nil>
	// correlation ID must be 1-128 ASCII letters, digits, dots, underscores, or hyphens
}
