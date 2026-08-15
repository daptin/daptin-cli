package mailprobe

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"
)

const imapMaxLine = 8192

type imapSession struct {
	conn net.Conn
	in   *bufio.Reader
	out  *bufio.Writer
	next int
}

func CheckIMAP(ctx context.Context, connection Connection) (IMAPResult, error) {
	started := time.Now()
	session, result, err := openIMAP(ctx, connection)
	if err != nil {
		return IMAPResult{}, err
	}
	defer session.conn.Close()
	_, _ = session.command("LOGOUT")
	result.Duration = time.Since(started)
	return result, nil
}

func SearchIMAP(ctx context.Context, request IMAPSearchRequest) (IMAPResult, error) {
	started := time.Now()
	if err := ValidateProtocolValue("IMAP username", request.Username, 255); err != nil {
		return IMAPResult{}, err
	}
	if err := ValidateProtocolValue("folder", request.Folder, 255); err != nil {
		return IMAPResult{}, err
	}
	if err := ValidateCorrelationID(request.CorrelationID); err != nil {
		return IMAPResult{}, err
	}
	if len(request.Password) == 0 || len(request.Password) > maxCredentialBytes {
		return IMAPResult{}, fmt.Errorf("IMAP credential is invalid")
	}
	if request.PollTimeout <= 0 || request.PollInterval <= 0 || request.PollInterval > request.PollTimeout {
		return IMAPResult{}, fmt.Errorf("IMAP polling durations are invalid")
	}

	session, result, err := openIMAP(ctx, request.Connection)
	if err != nil {
		return IMAPResult{}, err
	}
	defer session.conn.Close()
	if _, err := session.command("LOGIN " + imapQuote(request.Username) + " " + imapQuote(string(request.Password))); err != nil {
		return IMAPResult{}, protocolError("IMAP authentication failed")
	}
	result.Authenticated = true
	selectLines, err := session.command("SELECT " + imapQuote(request.Folder))
	if err != nil {
		return IMAPResult{}, protocolError("IMAP folder selection failed")
	}
	result.FolderSelected = true
	exists := mailboxExists(selectLines)
	if exists == 0 {
		statusLines, statusErr := session.command("STATUS " + imapQuote(request.Folder) + " (MESSAGES)")
		if statusErr == nil {
			exists = mailboxMessageCount(statusLines)
		}
	}
	slog.Debug("IMAP mailbox selected", "message_count", exists)

	searchDeadline := time.Now().Add(request.PollTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(searchDeadline) {
		searchDeadline = contextDeadline
	}
	_ = session.conn.SetDeadline(searchDeadline)
	messageID := "<" + request.CorrelationID + "@" + request.ServerName + ">"
	for {
		result.PollAttempts++
		lines, err := session.command("UID SEARCH HEADER Message-ID " + imapQuote(messageID))
		if err != nil {
			return IMAPResult{}, protocolError("IMAP correlation search failed")
		}
		if searchHasResult(lines) {
			result.MessageFound = true
			break
		}
		// Daptin v0.12.33 passes HEADER values to its database query as an
		// array and cannot match the scalar message_id column. Fall back to a
		// bounded sequence search and fetch only Message-ID headers. This still
		// proves visibility through IMAP and never retrieves message content.
		sequenceLines, err := session.command("SEARCH ALL")
		if err != nil {
			return IMAPResult{}, protocolError("IMAP correlation search failed")
		}
		sequences := searchNumbers(sequenceLines)
		if exists > 0 {
			sequences = sequences[:0]
			start := exists - 99
			if start < 1 {
				start = 1
			}
			for sequence := start; sequence <= exists; sequence++ {
				sequences = append(sequences, strconv.Itoa(sequence))
			}
		}
		if len(sequences) == 0 {
			// Some Daptin releases return an empty SEARCH/STATUS result even
			// when the selected mailbox contains visible rows. Probe at most
			// the first 100 sequence numbers; nonexistent sequences return no
			// header and the response remains bounded.
			for sequence := 100; sequence >= 1; sequence-- {
				sequences = append(sequences, strconv.Itoa(sequence))
			}
		}
		if len(sequences) > 100 {
			sequences = sequences[len(sequences)-100:]
		}
		slog.Debug("IMAP correlation fallback", "candidate_count", len(sequences))
		for i := len(sequences) - 1; i >= 0; i-- {
			headerLines, err := session.command("FETCH " + sequences[i] + " BODY.PEEK[HEADER.FIELDS (MESSAGE-ID)]")
			if err != nil {
				return IMAPResult{}, protocolError("IMAP correlation header check failed")
			}
			matched := messageIDPresent(headerLines, messageID)
			slog.Debug("IMAP correlation header checked", "response_parts", len(headerLines), "matched", matched)
			if matched {
				result.MessageFound = true
				break
			}
		}
		if result.MessageFound {
			break
		}
		remaining := time.Until(searchDeadline)
		if remaining <= 0 || remaining < request.PollInterval {
			return IMAPResult{}, protocolError("IMAP correlated message was not found")
		}
		timer := time.NewTimer(request.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return IMAPResult{}, protocolError("IMAP correlation search timed out")
		case <-timer.C:
		}
	}
	_, _ = session.command("LOGOUT")
	result.Duration = time.Since(started)
	return result, nil
}

func openIMAP(ctx context.Context, connection Connection) (*imapSession, IMAPResult, error) {
	if err := ValidateConnection(connection); err != nil {
		return nil, IMAPResult{}, err
	}
	conn, err := dialTCP(ctx, connection)
	if err != nil {
		return nil, IMAPResult{}, err
	}
	result := IMAPResult{Connected: true}
	if connection.TLSMode == TLSImplicit {
		tlsConn, tlsState, err := wrapTLS(ctx, conn, connection)
		if err != nil {
			return nil, IMAPResult{}, err
		}
		conn = tlsConn
		result.TLS = tlsState
	}
	session := newIMAPSession(conn)
	greeting, err := readBoundedLine(session.in, imapMaxLine)
	if err != nil || !strings.HasPrefix(strings.ToUpper(greeting), "* OK") {
		conn.Close()
		return nil, IMAPResult{}, protocolError("IMAP greeting was rejected")
	}
	if connection.TLSMode == TLSStartTLS {
		if _, err := session.command("STARTTLS"); err != nil {
			conn.Close()
			return nil, IMAPResult{}, protocolError("IMAP STARTTLS failed")
		}
		tlsConn, tlsState, err := wrapTLS(ctx, conn, connection)
		if err != nil {
			return nil, IMAPResult{}, err
		}
		session = newIMAPSession(tlsConn)
		result.TLS = tlsState
	}
	return session, result, nil
}

func newIMAPSession(conn net.Conn) *imapSession {
	return &imapSession{conn: conn, in: bufio.NewReaderSize(conn, imapMaxLine), out: bufio.NewWriter(conn), next: 1}
}

func (s *imapSession) command(command string) ([]string, error) {
	tag := fmt.Sprintf("d%04d", s.next)
	s.next++
	if _, err := s.out.WriteString(tag + " " + command + "\r\n"); err != nil {
		return nil, err
	}
	if err := s.out.Flush(); err != nil {
		return nil, err
	}
	lines := make([]string, 0, 4)
	for len(lines) < 128 {
		line, err := readBoundedLine(s.in, imapMaxLine)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(line, tag+" ") {
			fields := strings.Fields(line)
			if len(fields) < 2 || !strings.EqualFold(fields[1], "OK") {
				return nil, fmt.Errorf("IMAP command rejected")
			}
			return lines, nil
		}
		lines = append(lines, line)
		if literalSize, ok := imapLiteralSize(line); ok {
			if literalSize > 4096 {
				return nil, fmt.Errorf("IMAP literal is too large")
			}
			literal := make([]byte, literalSize)
			if _, err := io.ReadFull(s.in, literal); err != nil {
				return nil, err
			}
			lines = append(lines, string(literal))
			if _, err := readBoundedLine(s.in, imapMaxLine); err != nil {
				return nil, err
			}
		}
	}
	return nil, fmt.Errorf("IMAP response is too large")
}

func imapLiteralSize(line string) (int, bool) {
	open := strings.LastIndex(line, "{")
	if open < 0 || !strings.HasSuffix(line, "}") {
		return 0, false
	}
	size, err := strconv.Atoi(line[open+1 : len(line)-1])
	return size, err == nil && size >= 0
}

func imapQuote(value string) string {
	return strconv.Quote(value)
}

func searchHasResult(lines []string) bool {
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 2 && fields[0] == "*" && strings.EqualFold(fields[1], "SEARCH") {
			return true
		}
	}
	return false
}

func searchNumbers(lines []string) []string {
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "*" && strings.EqualFold(fields[1], "SEARCH") {
			results := make([]string, 0, len(fields)-2)
			for _, field := range fields[2:] {
				value, err := strconv.Atoi(field)
				if err == nil && value > 0 {
					results = append(results, field)
				}
			}
			return results
		}
	}
	return nil
}

func mailboxExists(lines []string) int {
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "*" && strings.EqualFold(fields[2], "EXISTS") {
			value, _ := strconv.Atoi(fields[1])
			return value
		}
	}
	return 0
}

func mailboxMessageCount(lines []string) int {
	for _, line := range lines {
		upper := strings.ToUpper(line)
		index := strings.Index(upper, "MESSAGES ")
		if index < 0 {
			continue
		}
		rest := line[index+len("MESSAGES "):]
		end := strings.IndexAny(rest, " )\r\n")
		if end >= 0 {
			rest = rest[:end]
		}
		value, _ := strconv.Atoi(rest)
		return value
	}
	return 0
}

func messageIDPresent(lines []string, expected string) bool {
	expected = strings.Trim(strings.TrimSpace(expected), "<>")
	for _, line := range lines {
		if colon := strings.Index(line, ":"); colon >= 0 && strings.EqualFold(strings.TrimSpace(line[:colon]), "message-id") {
			actual := strings.Trim(strings.TrimSpace(line[colon+1:]), "<>")
			if actual == expected {
				return true
			}
		}
	}
	return false
}
