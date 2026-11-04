package scan

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeClamd answers like clamd: it reads one INSTREAM and replies FOUND
// when the stream contains the EICAR marker.
func fakeClamd(t *testing.T, reply func(body []byte) string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer func() { _ = conn.Close() }()
				r := bufio.NewReader(conn)
				cmd, err := r.ReadString(0)
				if err != nil {
					return
				}
				if cmd == "zPING\x00" {
					_, _ = conn.Write([]byte("PONG\x00"))
					return
				}
				var body bytes.Buffer
				var header [4]byte
				for {
					if _, err := io.ReadFull(r, header[:]); err != nil {
						return
					}
					n := binary.BigEndian.Uint32(header[:])
					if n == 0 {
						break
					}
					if _, err := io.CopyN(&body, r, int64(n)); err != nil {
						return
					}
				}
				_, _ = conn.Write([]byte(reply(body.Bytes()) + "\x00"))
			}(conn)
		}
	}()
	return ln.Addr().String()
}

const eicar = `X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`

func TestClamdVerdicts(t *testing.T) {
	addr := fakeClamd(t, func(body []byte) string {
		if bytes.Contains(body, []byte("EICAR")) {
			return "stream: Win.Test.EICAR_HDB-1 FOUND"
		}
		if len(body) > 1<<20 {
			return "INSTREAM size limit exceeded. ERROR"
		}
		return "stream: OK"
	})
	c := NewClamd(addr, 5*time.Second)
	c.ChunkSize = 7 // force several chunks
	ctx := context.Background()

	if err := c.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if err := c.Scan(ctx, strings.NewReader("an ordinary invoice")); err != nil {
		t.Fatalf("clean file: %v", err)
	}
	err := c.Scan(ctx, strings.NewReader(eicar))
	var infected *Infected
	if !errors.As(err, &infected) || infected.Signature != "Win.Test.EICAR_HDB-1" || !errors.Is(err, ErrInfected) {
		t.Fatalf("expected EICAR to be found, got %v", err)
	}
	c.ChunkSize = 64 << 10
	err = c.Scan(ctx, bytes.NewReader(make([]byte, 2<<20)))
	if err == nil || errors.Is(err, ErrInfected) {
		t.Fatalf("expected a scan error for an oversize stream, got %v", err)
	}
}

func TestClamdUnreachable(t *testing.T) {
	c := NewClamd("127.0.0.1:1", time.Second)
	if err := c.Scan(context.Background(), strings.NewReader("x")); err == nil || errors.Is(err, ErrInfected) {
		t.Fatalf("expected a connection error, got %v", err)
	}
}
