package server_test

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"socks4/client"
	"socks4/proto"

	"github.com/stretchr/testify/require"
)

func newClient(t *testing.T) *client.Client {
	t.Helper()

	s := createServer(t)

	addr, err := s.ListenAndServe("localhost:0")
	require.NoError(t, err)
	require.NotNil(t, addr)

	client := client.NewClient(addr.String(), "")
	require.NotNil(t, client)

	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return client
}

func newEchoServer(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	go func() {
		for {
			accepted, err := ln.Accept()
			if errors.Is(err, net.ErrClosed) {
				return
			}
			require.NoError(t, err)

			go echo(t, accepted)
		}
	}()

	t.Cleanup(func() { require.NoError(t, ln.Close()) })

	return ln.Addr().String()
}

func echo(t *testing.T, conn net.Conn) {
	t.Helper()

	defer func() { require.NoError(t, conn.Close()) }()

	buff := make([]byte, 256)
	n, err := conn.Read(buff)
	if errors.Is(err, io.EOF) {
		return
	}
	require.NoError(t, err)

	wn, err := conn.Write(buff[:n])
	require.NoError(t, err)
	require.Equal(t, n, wn)
}

func requireClosed(t *testing.T, conn net.Conn) {
	t.Helper()

	n, err := conn.Read([]byte{0})
	require.ErrorIs(t, err, io.EOF)
	require.Zero(t, n)
}

func writePacket(t *testing.T, client *client.Client, packet []byte) {
	t.Helper()

	n, err := client.Write(packet)
	require.NoError(t, err)
	require.Equal(t, len(packet), n)
}

func TestConnects(t *testing.T) {
	t.Parallel()

	client := newClient(t)

	err := client.Connect("127.0.0.1:80")
	require.Error(t, err)

	require.NotNil(t, client.RemoteAddr())
	require.NotEmpty(t, client.RemoteAddr().String())
}

func TestTimeout(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.SkipNow()
	}

	client := newClient(t)
	client.Write([]byte{})

	time.Sleep(time.Second * 121) // 2min 1sec

	requireClosed(t, client)
}

func TestBadRequest(t *testing.T) {
	t.Parallel()

	t.Run("ShortRead", func(t *testing.T) {
		t.Parallel()
		client := newClient(t)

		writePacket(t, client, []byte{proto.Version, 0, 0})

		requireClosed(t, client)
	})

	t.Run("BadVersion", func(t *testing.T) {
		t.Parallel()
		client := newClient(t)

		writePacket(t, client, []byte{proto.Version + 1, 0, 0, 0, 0, 0, 0, 0, 0})

		requireClosed(t, client)
	})

	t.Run("BadCommand", func(t *testing.T) {
		t.Parallel()
		client := newClient(t)

		writePacket(t, client, []byte{proto.Version, 0, 1, 2, 3, 4, 5, 6, 0})

		resp, err := proto.ReadReply(client)
		require.NoError(t, err)
		require.Equal(t, proto.ErrorReply, resp.Code())

		requireClosed(t, client)
	})

	t.Run("UserTooLong", func(t *testing.T) {
		t.Parallel()
		client := newClient(t)

		buff := bytes.NewBuffer([]byte{4, proto.ConnectCommand, 1, 2, 3, 4, 5, 6})
		_, err := buff.Write(bytes.Repeat([]byte{'a'}, 64))
		require.NoError(t, err)
		require.NoError(t, buff.WriteByte(0))

		writePacket(t, client, buff.Bytes())

		requireClosed(t, client)
	})
}

func TestUnreachableRemote(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip()
	}

	client := newClient(t)

	// "connect" to unreachable 240.0.0.0:80
	err := client.Connect("240.0.0.0:80")
	require.Error(t, err)

	requireClosed(t, client)
}

func TestConnectionRefused(t *testing.T) {
	t.Parallel()

	client := newClient(t)

	err := client.Connect("127.0.0.1:80")
	require.Error(t, err)

	requireClosed(t, client)
}

func TestRemoteBindTimeout(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.SkipNow()
	}

	client := newClient(t)

	// bind, allow localhost:8000 (random, doesn't matter for the test)
	err := client.Bind("127.0.0.1:0", func(boundAddress string) error {
		t.Logf("Server bound to %s", boundAddress)
		return nil
	})
	require.Error(t, err)

	requireClosed(t, client)
}

func TestMismatchRemote(t *testing.T) {
	t.Parallel()

	client := newClient(t)

	// "expect" a connection from 1.2.3.4:8000
	err := client.Bind("1.2.3.4:8000", func(addr string) error {
		t.Logf("Server bound to %s", addr)
		conn, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		requireClosed(t, conn)
		return nil
	})
	require.Error(t, err)
	requireClosed(t, client)
}

func TestConnectExchange(t *testing.T) {
	t.Parallel()

	client := newClient(t)
	echoServer := newEchoServer(t)

	err := client.Connect(echoServer)
	require.NoError(t, err)

	message := "hello world"
	buff := []byte(message)
	n, err := client.Write(buff)
	require.NoError(t, err)
	require.Equal(t, len(message), n)

	n, err = client.Read(buff)
	require.NoError(t, err)
	require.Equal(t, len(message), n)
	require.EqualValues(t, message, buff)
}

func TestBindExchange(t *testing.T) {
	t.Parallel()

	client := newClient(t)

	client.Bind("127.0.0.1:0", func(addr string) error {
		remote, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		go echo(t, remote)
		return nil
	})

	message := "hello world"
	buff := []byte(message)
	n, err := client.Write(buff)
	require.NoError(t, err)
	require.Equal(t, len(message), n)

	n, err = client.Read(buff)
	require.NoError(t, err)
	require.Equal(t, len(message), n)
	require.EqualValues(t, message, buff)

	requireClosed(t, client)
}

func TestExchangeTimeout(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.SkipNow()
	}

	client := newClient(t)
	echoServer := newEchoServer(t)

	err := client.Connect(echoServer)
	require.NoError(t, err)

	time.Sleep(time.Second * 31)

	requireClosed(t, client)
}
