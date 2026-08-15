package mailprobe

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const smtpMaxLine = 4096

type smtpSession struct {
	conn net.Conn
	in   *bufio.Reader
	out  *bufio.Writer
}

func CheckSMTP(ctx context.Context, connection Connection) (SMTPCheckResult, error) {
	started := time.Now()
	session, result, err := openSMTP(ctx, connection)
	if err != nil {
		return SMTPCheckResult{}, err
	}
	defer session.conn.Close()
	_ = session.command(221, "QUIT")
	result.Duration = time.Since(started)
	return result, nil
}

func DeliverSMTP(ctx context.Context, request SMTPDeliveryRequest) (SMTPDeliveryResult, error) {
	started := time.Now()
	if err := ValidateProtocolValue("SMTP username", request.Username, 255); err != nil {
		return SMTPDeliveryResult{}, err
	}
	if err := ValidateProtocolValue("sender", request.From, 255); err != nil {
		return SMTPDeliveryResult{}, err
	}
	if err := ValidateProtocolValue("recipient", request.To, 255); err != nil {
		return SMTPDeliveryResult{}, err
	}
	if err := ValidateCorrelationID(request.CorrelationID); err != nil {
		return SMTPDeliveryResult{}, err
	}
	if len(request.Password) == 0 || len(request.Password) > maxCredentialBytes {
		return SMTPDeliveryResult{}, fmt.Errorf("SMTP credential is invalid")
	}

	session, check, err := openSMTP(ctx, request.Connection)
	if err != nil {
		return SMTPDeliveryResult{}, err
	}
	defer session.conn.Close()
	result := SMTPDeliveryResult{SMTPCheckResult: check}
	if !check.AuthLoginAdvertised {
		return SMTPDeliveryResult{}, protocolError("SMTP AUTH LOGIN is unavailable")
	}

	if err := session.command(334, "AUTH LOGIN"); err != nil {
		return SMTPDeliveryResult{}, protocolError("SMTP authentication failed")
	}
	if err := session.command(334, base64.StdEncoding.EncodeToString([]byte(request.Username))); err != nil {
		return SMTPDeliveryResult{}, protocolError("SMTP authentication failed")
	}
	if err := session.command(235, base64.StdEncoding.EncodeToString(request.Password)); err != nil {
		return SMTPDeliveryResult{}, protocolError("SMTP authentication failed")
	}
	result.Authenticated = true
	if err := session.command(250, "MAIL FROM:<"+request.From+">"); err != nil {
		return SMTPDeliveryResult{}, protocolError("SMTP sender rejected")
	}
	result.SenderAccepted = true
	if err := session.command(250, "RCPT TO:<"+request.To+">"); err != nil {
		return SMTPDeliveryResult{}, protocolError("SMTP recipient rejected")
	}
	result.RecipientAccepted = true
	if err := session.command(354, "DATA"); err != nil {
		return SMTPDeliveryResult{}, protocolError("SMTP message rejected")
	}
	messageID := "<" + request.CorrelationID + "@" + request.ServerName + ">"
	message := strings.Join([]string{
		"From: " + request.From,
		"To: " + request.To,
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		"Message-ID: " + messageID,
		"Subject: Daptin mail qualification",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Daptin mail qualification message.",
		".",
		"",
	}, "\r\n")
	if _, err := session.out.WriteString(message); err != nil {
		return SMTPDeliveryResult{}, protocolError("SMTP message write failed")
	}
	if err := session.out.Flush(); err != nil {
		return SMTPDeliveryResult{}, protocolError("SMTP message write failed")
	}
	if _, _, err := session.readResponse(250); err != nil {
		return SMTPDeliveryResult{}, protocolError("SMTP message was not accepted")
	}
	result.MessageAccepted = true
	_ = session.command(221, "QUIT")
	result.Duration = time.Since(started)
	return result, nil
}

func openSMTP(ctx context.Context, connection Connection) (*smtpSession, SMTPCheckResult, error) {
	if err := ValidateConnection(connection); err != nil {
		return nil, SMTPCheckResult{}, err
	}
	conn, err := dialTCP(ctx, connection)
	if err != nil {
		return nil, SMTPCheckResult{}, err
	}
	result := SMTPCheckResult{Connected: true}
	if connection.TLSMode == TLSImplicit {
		tlsConn, tlsState, err := wrapTLS(ctx, conn, connection)
		if err != nil {
			return nil, SMTPCheckResult{}, err
		}
		conn = tlsConn
		result.TLS = tlsState
	}
	session := newSMTPSession(conn)
	if _, _, err := session.readResponse(220); err != nil {
		conn.Close()
		return nil, SMTPCheckResult{}, protocolError("SMTP greeting was rejected")
	}
	_, capabilities, err := session.exchange(250, "EHLO daptin-cli.invalid")
	if err != nil {
		conn.Close()
		return nil, SMTPCheckResult{}, protocolError("SMTP EHLO failed")
	}
	result.SMTPReady = true
	result.StartTLSAdvertised = capabilityPresent(capabilities, "STARTTLS")
	result.AuthLoginAdvertised = authLoginPresent(capabilities)

	if connection.TLSMode == TLSStartTLS {
		if !result.StartTLSAdvertised {
			conn.Close()
			return nil, SMTPCheckResult{}, protocolError("SMTP STARTTLS is unavailable")
		}
		if err := session.command(220, "STARTTLS"); err != nil {
			conn.Close()
			return nil, SMTPCheckResult{}, protocolError("SMTP STARTTLS failed")
		}
		tlsConn, tlsState, err := wrapTLS(ctx, conn, connection)
		if err != nil {
			return nil, SMTPCheckResult{}, err
		}
		session = newSMTPSession(tlsConn)
		result.TLS = tlsState
		_, capabilities, err = session.exchange(250, "EHLO daptin-cli.invalid")
		if err != nil {
			tlsConn.Close()
			return nil, SMTPCheckResult{}, protocolError("SMTP EHLO after STARTTLS failed")
		}
		result.AuthLoginAdvertised = authLoginPresent(capabilities)
	}
	return session, result, nil
}

func newSMTPSession(conn net.Conn) *smtpSession {
	return &smtpSession{conn: conn, in: bufio.NewReaderSize(conn, smtpMaxLine), out: bufio.NewWriter(conn)}
}

func (s *smtpSession) command(expected int, command string) error {
	_, _, err := s.exchange(expected, command)
	return err
}

func (s *smtpSession) exchange(expected int, command string) (int, []string, error) {
	if _, err := s.out.WriteString(command + "\r\n"); err != nil {
		return 0, nil, err
	}
	if err := s.out.Flush(); err != nil {
		return 0, nil, err
	}
	return s.readResponse(expected)
}

func (s *smtpSession) readResponse(expected int) (int, []string, error) {
	lines := make([]string, 0, 4)
	for len(lines) < 64 {
		line, err := readBoundedLine(s.in, smtpMaxLine)
		if err != nil {
			return 0, nil, err
		}
		if len(line) < 4 || line[0] < '0' || line[0] > '9' || line[1] < '0' || line[1] > '9' || line[2] < '0' || line[2] > '9' {
			return 0, nil, fmt.Errorf("malformed SMTP response")
		}
		code := int(line[0]-'0')*100 + int(line[1]-'0')*10 + int(line[2]-'0')
		lines = append(lines, line[4:])
		if line[3] == ' ' {
			if code != expected {
				return code, nil, fmt.Errorf("unexpected SMTP response")
			}
			return code, lines, nil
		}
		if line[3] != '-' {
			return 0, nil, fmt.Errorf("malformed SMTP response")
		}
	}
	return 0, nil, fmt.Errorf("SMTP response is too large")
}

func readBoundedLine(reader *bufio.Reader, max int) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return "", io.ErrUnexpectedEOF
		}
		return "", err
	}
	if len(line) > max {
		return "", fmt.Errorf("protocol response line is too large")
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

func capabilityPresent(lines []string, name string) bool {
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.EqualFold(fields[0], name) {
			return true
		}
	}
	return false
}

func authLoginPresent(lines []string) bool {
	for _, line := range lines {
		fields := strings.Fields(strings.ToUpper(line))
		if len(fields) > 1 && fields[0] == "AUTH" {
			for _, mechanism := range fields[1:] {
				if mechanism == "LOGIN" {
					return true
				}
			}
		}
	}
	return false
}
