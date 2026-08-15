package mailprobe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"time"
)

func dialTCP(ctx context.Context, c Connection) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: c.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.Address)
	if err != nil {
		return nil, protocolError("mail listener connection failed")
	}
	if err := conn.SetDeadline(deadlineFromContext(ctx, c.Timeout)); err != nil {
		conn.Close()
		return nil, protocolError("mail listener deadline setup failed")
	}
	return conn, nil
}

func tlsConfig(c Connection) (*tls.Config, error) {
	config := &tls.Config{ServerName: c.ServerName, MinVersion: tls.VersionTLS12}
	if c.CAFile == "" {
		return config, nil
	}
	pem, err := os.ReadFile(c.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("CA file contains no certificates")
	}
	config.RootCAs = roots
	return config, nil
}

func wrapTLS(ctx context.Context, conn net.Conn, c Connection) (*tls.Conn, TLSResult, error) {
	config, err := tlsConfig(c)
	if err != nil {
		return nil, TLSResult{}, err
	}
	tlsConn := tls.Client(conn, config)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, TLSResult{}, protocolError("mail TLS verification failed")
	}
	return tlsConn, tlsResult(tlsConn.ConnectionState()), nil
}

func deadlineFromContext(ctx context.Context, fallbackDuration time.Duration) time.Time {
	if value, ok := ctx.Deadline(); ok {
		return value
	}
	return time.Now().Add(fallbackDuration)
}
