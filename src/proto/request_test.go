package proto_test

import (
	"math/rand"
	"net"
	"strings"
	"testing"

	"socks4/proto"

	"github.com/stretchr/testify/require"
)

func relay[T any](t *testing.T, f func(net.Conn) (*T, error), packet []byte) (*T, error) {
	t.Helper()

	client, conn := net.Pipe()
	defer client.Close()
	defer conn.Close()

	go func() {
		n, err := client.Write(packet)
		require.Equal(t, len(packet), n)
		require.NoError(t, err)
	}()

	return f(conn)
}

func TestNewRequest(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		remote string
		user   string
	}{
		{"something bad", ""},
		{":5", ""},
		{"localhost:", ""},
		{"localhost:80", ""},
		{"localhost:num", ""},
		{"1.1.1.1:tmp", ""},
		{"1.1.1.1:80", strings.Repeat("A", 64)},
	} {
		t.Run(test.remote+"_"+test.user, func(remote, user string) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				req, err := proto.NewRequest(proto.InvalidCommand, remote, user)
				require.Error(t, err)
				require.Nil(t, req)
			}
		}(test.remote, test.user))
	}

	req, err := proto.NewRequest(proto.InvalidCommand, "1.1.1.1:1", "")
	require.NoError(t, err)
	require.NotNil(t, req)
}

func TestReadRequest(t *testing.T) {
	t.Parallel()

	t.Run("BadRead", func(t *testing.T) {
		t.Parallel()

		client, conn := net.Pipe()
		defer client.Close()
		conn.Close() // cause an error

		r, err := proto.ReadRequest(conn)
		require.Nil(t, r)
		require.ErrorContains(t, err, "failed to read from connection")
	})

	t.Run("TooShort", func(t *testing.T) {
		t.Parallel()

		r, err := relay(t, proto.ReadRequest, []byte{})
		require.Nil(t, r)
		require.ErrorContains(t, err, "failed to read entire request")
	})

	t.Run("TooLong", func(t *testing.T) {
		t.Parallel()

		r, err := relay(t, proto.ReadRequest, make([]byte, 72))
		require.NotNil(t, r)
		require.NoError(t, err)

		r, err = relay(t, proto.ReadRequest, make([]byte, 73))
		require.Nil(t, r)
		require.ErrorContains(t, err, "request is too long")
	})

	t.Run("Ok", func(t *testing.T) {
		t.Parallel()

		r, err := relay(t, proto.ReadRequest, []byte{4, 0, 0, 0, 0, 0, 0, 0, 0})
		require.NotNil(t, r)
		require.NoError(t, err)
	})
}

func TestRequestVersion(t *testing.T) {
	t.Parallel()

	r, err := proto.NewRequest(proto.ConnectCommand, "127.0.0.1:80", "")
	require.NotNil(t, r)
	require.NoError(t, err)

	require.Equal(t, proto.Version, r.Version())
}

func TestRequestCommand(t *testing.T) {
	t.Parallel()

	for name, command := range map[string]byte{
		"Invalid": proto.InvalidCommand,
		"Connect": proto.ConnectCommand,
		"Bind":    proto.BindCommand,
	} {
		t.Run(name, func(command byte) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				r, err := proto.NewRequest(command, "127.0.0.1:80", "")
				require.NotNil(t, r)
				require.NoError(t, err)

				require.EqualValues(t, command, r.Command())
			}
		}(command))
	}

	t.Run("random", func(t *testing.T) {
		t.Parallel()

		v := rand.Intn(256-int(proto.BindCommand)) + int(proto.BindCommand)

		r, err := proto.NewRequest(proto.Command(v), "127.0.0.1:80", "")
		require.NotNil(t, r)
		require.NoError(t, err)

		require.EqualValues(t, proto.InvalidCommand, r.Command())
	})
}

func TestRequestPort(t *testing.T) {
	t.Parallel()

	r, err := proto.NewRequest(proto.ConnectCommand, "127.0.0.1:80", "")
	require.NotNil(t, r)
	require.NoError(t, err)

	require.Equal(t, 80, r.Port())
}

func TestRequestIP(t *testing.T) {
	t.Parallel()

	r, err := proto.NewRequest(proto.ConnectCommand, "127.0.0.1:80", "")
	require.NotNil(t, r)
	require.NoError(t, err)

	require.True(t, net.IPv4(127, 0, 0, 1).Equal(r.IP()), "expected IP to be 127.0.0.1")
}

func TestRequestAddress(t *testing.T) {
	t.Parallel()

	r, err := proto.NewRequest(proto.ConnectCommand, "127.0.0.1:80", "")
	require.NotNil(t, r)
	require.NoError(t, err)

	require.Equal(t, "127.0.0.1:80", r.Address())
}

func TestRequestUserID(t *testing.T) {
	t.Parallel()

	r, err := proto.NewRequest(proto.ConnectCommand, "127.0.0.1:80", "")
	require.NotNil(t, r)
	require.NoError(t, err)

	require.Empty(t, r.UserID())

	r, err = proto.NewRequest(proto.ConnectCommand, "127.0.0.1:80", "mcr")
	require.NotNil(t, r)
	require.NoError(t, err)

	require.Equal(t, "mcr", r.UserID())
}

func TestRequestSerialize(t *testing.T) {
	t.Parallel()

	req, err := proto.NewRequest(proto.ConnectCommand, "127.0.0.1:80", "")
	require.NoError(t, err)
	require.NotNil(t, req)

	require.Equal(t, []byte{proto.Version, proto.ConnectCommand, 0, 80, 127, 0, 0, 1, 0}, req.Serialize())
}
