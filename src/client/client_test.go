package client_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"socks4/client"
	"socks4/server"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupEcho(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ln.Close()) })

	go func() {
		for {
			conn, err := ln.Accept()
			if errors.Is(err, net.ErrClosed) {
				return
			}
			require.NoError(t, err)
			go echo(t, conn)
		}
	}()

	return ln.Addr().String()
}

func echo(t *testing.T, conn net.Conn) {
	buff := make([]byte, 256)
	n, err := conn.Read(buff)
	if errors.Is(err, io.EOF) {
		return
	}
	require.NoError(t, err)
	wn, err := conn.Write(buff[:n])
	require.NoError(t, err)
	require.Equal(t, n, wn)
	conn.Close()
}

func setupProxy(t *testing.T) string {
	t.Helper()

	s := server.NewServer(zaptest.NewLogger(t))
	require.NotNil(t, s)

	addr, err := s.ListenAndServe("localhost:0")
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		s.Close(ctx)
		cancel()
	})

	return addr.String()
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	client := client.NewClient("localhost:0", "")
	require.NotNil(t, client)
}

func TestConnect(t *testing.T) {
	t.Parallel()

	echoServer := setupEcho(t)
	proxyServer := setupProxy(t)

	for _, test := range []struct {
		remote string
		user   string
	}{
		{echoServer, strings.Repeat("a", 64)},
		{"0.0.0.0", ""},
		{"localhost:80", ""},
	} {
		t.Run(test.remote+"_"+test.user, func(remote, user string) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				c := client.NewClient(proxyServer, user)

				err := c.Connect(remote)
				require.Error(t, err)
			}
		}(test.remote, test.user))
	}

	c := client.NewClient(proxyServer, "")
	require.NotNil(t, c)
	require.NoError(t, c.Connect(echoServer))

	msg := "hello world"
	buff := []byte(msg)
	n, err := c.Write(buff)
	require.NoError(t, err)
	require.Equal(t, len(buff), n)

	n, err = c.Read(buff)
	require.NoError(t, err)
	require.Equal(t, len(buff), n)
	require.EqualValues(t, msg, buff)
}

func TestBind(t *testing.T) {
	t.Parallel()

	proxyServer := setupProxy(t)

	c := client.NewClient(proxyServer, "")
	require.NotNil(t, c)

	err := c.Bind("127.0.0.1:0", func(boundAt string) error {
		remote, err := net.Dial("tcp", boundAt)
		if err != nil {
			return fmt.Errorf("failed to dial remote - %w", err)
		}
		go echo(t, remote)
		return nil
	})
	require.NoError(t, err)

	msg := "hello world"
	n, err := c.Write([]byte(msg))
	require.NoError(t, err)
	require.Equal(t, len(msg), n)

	buff := make([]byte, len(msg))
	n, err = c.Read(buff)
	require.NoError(t, err)
	require.Equal(t, len(msg), n)
	require.EqualValues(t, msg, buff)
}
