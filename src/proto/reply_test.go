package proto_test

import (
	"encoding/binary"
	"math/rand"
	"net"
	"strconv"
	"testing"

	"socks4/proto"

	"github.com/stretchr/testify/require"
)

func TestNewReply(t *testing.T) {
	t.Parallel()

	port := 0
	ip := net.IPv4(0, 0, 0, 0)
	r := proto.NewReply(proto.SuccessReply, ip, port)
	require.NotNil(t, r)
}

func TestReadReply(t *testing.T) {
	t.Parallel()

	t.Run("BadRead", func(t *testing.T) {
		t.Parallel()

		client, conn := net.Pipe()
		defer client.Close()
		conn.Close() // cause an error

		r, err := proto.ReadReply(conn)
		require.Nil(t, r)
		require.ErrorContains(t, err, "failed to read from connection")
	})

	t.Run("TooShort", func(t *testing.T) {
		t.Parallel()

		r, err := relay(t, proto.ReadReply, []byte{})
		require.Nil(t, r)
		require.ErrorContains(t, err, "failed to read entire reply")
	})

	t.Run("TooLong", func(t *testing.T) {
		t.Parallel()

		r, err := relay(t, proto.ReadReply, make([]byte, 8))
		require.NotNil(t, r)
		require.NoError(t, err)

		r, err = relay(t, proto.ReadReply, make([]byte, 9))
		require.Nil(t, r)
		require.ErrorContains(t, err, "reply is too long")
	})

	t.Run("Ok", func(t *testing.T) {
		t.Parallel()

		r, err := relay(t, proto.ReadReply, []byte{4, 0, 0, 0, 0, 0, 0, 0})
		require.NotNil(t, r)
		require.NoError(t, err)
	})
}

func TestReplyVersion(t *testing.T) {
	t.Parallel()

	reply := proto.NewReply(proto.SuccessReply, net.IPv4(0, 0, 0, 0), 0)
	require.NotNil(t, reply)

	require.Equal(t, proto.Version, reply.Version())
}

func TestReplyCode(t *testing.T) {
	t.Parallel()

	for _, code := range []proto.ReplyCode{
		proto.SuccessReply,
		proto.ErrorReply,
	} {
		t.Run(strconv.Itoa(int(code)), func(code byte) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				reply := proto.NewReply(code, net.IPv4(0, 0, 0, 0), 0)
				require.NotNil(t, reply)

				require.Equal(t, code, reply.Code())
			}
		}(code))
	}

	t.Run("random", func(t *testing.T) {
		t.Parallel()
		rand := byte(rand.Intn(256-int(proto.ErrorReply)) + int(proto.ErrorReply))
		reply := proto.NewReply(rand, net.IPv4(0, 0, 0, 0), 0)
		require.NotNil(t, reply)

		require.Equal(t, proto.InvalidReply, reply.Code())
	})
}

func TestReplyPort(t *testing.T) {
	t.Parallel()

	port := rand.Intn(65536)
	reply := proto.NewReply(proto.SuccessReply, net.IPv4(0, 0, 0, 0), port)
	require.NotNil(t, reply)

	require.Equal(t, port, reply.Port())
}

func TestReplyIP(t *testing.T) {
	t.Parallel()

	a := byte(rand.Intn(256))
	b := byte(rand.Intn(256))
	c := byte(rand.Intn(256))
	d := byte(rand.Intn(256))
	ip := net.IPv4(a, b, c, d)

	reply := proto.NewReply(proto.SuccessReply, ip, 0)
	require.NotNil(t, reply)

	require.Equal(t, ip, reply.IP())
}

func TestReplyAddress(t *testing.T) {
	t.Parallel()

	port := rand.Intn(65536)
	a := byte(rand.Intn(256))
	b := byte(rand.Intn(256))
	c := byte(rand.Intn(256))
	d := byte(rand.Intn(256))
	ip := net.IPv4(a, b, c, d)

	reply := proto.NewReply(proto.SuccessReply, ip, port)
	require.NotNil(t, reply)

	require.Equal(t, (&net.TCPAddr{IP: ip, Port: port}).String(), reply.Address())
}

func TestReplySerialize(t *testing.T) {
	t.Parallel()

	for name, code := range map[string]byte{
		"SuccessReply": proto.SuccessReply,
		"ErrorReply":   proto.ErrorReply,
	} {
		t.Run(name, func(code byte) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				port := rand.Intn(65536)
				a := byte(rand.Intn(256))
				b := byte(rand.Intn(256))
				c := byte(rand.Intn(256))
				d := byte(rand.Intn(256))
				ip := net.IPv4(a, b, c, d)

				r := proto.NewReply(code, ip, port)
				require.NotNil(t, r)

				data := r.Serialize()
				require.NotEmpty(t, data)

				require.EqualValues(t, proto.Version, data[0])
				require.EqualValues(t, code, data[1])
				require.EqualValues(t, port, binary.BigEndian.Uint16(data[2:4]))
				require.EqualValues(t, ip.To4(), data[4:8])
			}
		}(code))
	}
}
