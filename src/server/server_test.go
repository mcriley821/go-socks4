package server_test

import (
	"context"
	"net"
	"testing"
	"time"

	"socks4/server"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func createServer(t *testing.T) *server.Server {
	t.Helper()

	s := server.NewServer(zaptest.NewLogger(t))
	require.NotNil(t, s)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		require.NoError(t, s.Close(ctx))
		cancel()
	})

	return s
}

func TestNewServer(t *testing.T) {
	t.Parallel()

	_ = createServer(t)
}

func TestListenAndServe(t *testing.T) {
	t.Parallel()

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		s := createServer(t)

		addr, err := s.ListenAndServe("localhost:0")
		require.NotNil(t, addr)
		require.NoError(t, err)
	})

	t.Run("BadEndpoint", func(t *testing.T) {
		t.Parallel()

		s := createServer(t)

		addr, err := s.ListenAndServe("8.8.8.8:0")
		require.Nil(t, addr)
		require.Error(t, err)
	})

	t.Run("Serves", func(t *testing.T) {
		t.Parallel()

		s := createServer(t)

		addr, err := s.ListenAndServe("localhost:0")
		require.NotNil(t, addr)
		require.NoError(t, err)

		conn, err := net.Dial("tcp", addr.String())
		require.NoError(t, err)
		require.NotNil(t, conn)
		require.NoError(t, conn.Close())
	})
}

func TestShutdown(t *testing.T) {
	t.Parallel()

	t.Run("ctxError", func(t *testing.T) {
		t.Parallel()

		s := server.NewServer(zaptest.NewLogger(t))
		require.NotNil(t, s)

		_, err := s.ListenAndServe("localhost:0")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), time.Microsecond)
		time.Sleep(time.Millisecond)
		require.Error(t, s.Close(ctx))
		cancel()
	})

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		s := createServer(t)
		require.NotNil(t, s)
	})
}
