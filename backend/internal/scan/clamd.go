// Package scan checks files for malware before they are stored.
package scan

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// ErrInfected wraps the name of the malware the scanner found.
var ErrInfected = errors.New("file contains malware")

// Infected reports what was found.
type Infected struct{ Signature string }

func (e *Infected) Error() string { return "file contains malware: " + e.Signature }
func (e *Infected) Unwrap() error { return ErrInfected }

// Scanner checks a stream. It returns *Infected when the stream is unsafe and
// another error when it could not decide.
type Scanner interface {
	Scan(ctx context.Context, r io.Reader) error
	Ping(ctx context.Context) error
}

// Clamd talks to a ClamAV daemon over TCP using INSTREAM, so files never
// need to be on a disk clamd can read.
type Clamd struct {
	Addr    string
	Timeout time.Duration
	// ChunkSize is how much is sent per INSTREAM chunk.
	ChunkSize int
}

func NewClamd(addr string, timeout time.Duration) *Clamd {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return &Clamd{Addr: addr, Timeout: timeout, ChunkSize: 64 << 10}
}

func (c *Clamd) dial(ctx context.Context) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return nil, fmt.Errorf("connect to clamd: %w", err)
	}
	return conn, nil
}

// Ping checks the daemon answers.
func (c *Clamd) Ping(ctx context.Context) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write([]byte("zPING\x00")); err != nil {
		return err
	}
	reply, err := readReply(conn)
	if err != nil {
		return err
	}
	if reply != "PONG" {
		return fmt.Errorf("unexpected clamd reply %q", reply)
	}
	return nil
}

// Scan streams r to clamd and interprets its verdict.
func (c *Clamd) Scan(ctx context.Context, r io.Reader) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	deadline := time.Now().Add(c.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	w := bufio.NewWriter(conn)
	if _, err := w.WriteString("zINSTREAM\x00"); err != nil {
		return err
	}
	size := c.ChunkSize
	if size <= 0 {
		size = 64 << 10
	}
	buf := make([]byte, size)
	var header [4]byte
	for {
		n, readErr := r.Read(buf)
		if n > 0 {
			binary.BigEndian.PutUint32(header[:], uint32(n))
			if _, err := w.Write(header[:]); err != nil {
				return err
			}
			if _, err := w.Write(buf[:n]); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	binary.BigEndian.PutUint32(header[:], 0)
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		// clamd closes the connection early when the stream exceeds its
		// StreamMaxLength; its reply explains why.
		if reply, replyErr := readReply(conn); replyErr == nil {
			return verdict(reply)
		}
		return err
	}
	reply, err := readReply(conn)
	if err != nil {
		return err
	}
	return verdict(reply)
}

func readReply(conn net.Conn) (string, error) {
	data, err := bufio.NewReader(conn).ReadBytes(0)
	if err != nil && len(data) == 0 {
		return "", fmt.Errorf("read clamd reply: %w", err)
	}
	return strings.TrimSpace(string(bytes.TrimRight(data, "\x00"))), nil
}

// verdict reads replies such as "stream: OK",
// "stream: Win.Test.EICAR_HDB-1 FOUND" and "INSTREAM size limit exceeded. ERROR".
func verdict(reply string) error {
	reply = strings.TrimPrefix(reply, "stream: ")
	switch {
	case reply == "OK":
		return nil
	case strings.HasSuffix(reply, " FOUND"):
		return &Infected{Signature: strings.TrimSuffix(reply, " FOUND")}
	case strings.HasSuffix(reply, " ERROR"):
		return fmt.Errorf("clamd: %s", strings.TrimSuffix(reply, " ERROR"))
	}
	return fmt.Errorf("unexpected clamd reply %q", reply)
}
