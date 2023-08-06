package proto

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

type Reply struct {
	//  version byte
	//  reply   byte
	//  dstPort uint16 BIG
	//  dstAddr uint32 BIG
	raw []byte
}

const (
	maxReplySize = 8
)

type ReplyCode = byte

var (
	InvalidReply ReplyCode = 0
	SuccessReply ReplyCode = 90
	ErrorReply   ReplyCode = 91
)

func NewReply(code ReplyCode, ip net.IP, port int) *Reply {
	buf := make([]byte, maxReplySize)
	buf[0] = Version
	buf[1] = code
	binary.BigEndian.PutUint16(buf[2:4], uint16(port))
	copy(buf[4:], ip.To4())
	return &Reply{raw: buf}
}

func ReadReply(conn net.Conn) (*Reply, error) {
	buf := make([]byte, maxReplySize+1)
	if n, err := conn.Read(buf); err != nil {
		return nil, fmt.Errorf("failed to read from connection - %w", err)
	} else if n < maxReplySize {
		return nil, errors.New("failed to read entire reply")
	} else if n > maxReplySize {
		return nil, errors.New("reply is too long")
	}
	return &Reply{raw: buf}, nil
}

func (r Reply) Version() int {
	return int(r.raw[0])
}

func (r Reply) Code() ReplyCode {
	switch r.raw[1] {
	case SuccessReply:
		return SuccessReply
	case ErrorReply:
		return ErrorReply
	default:
		return InvalidReply
	}
}

func (r Reply) Port() int {
	return int(binary.BigEndian.Uint16(r.raw[2:4]))
}

func (r Reply) IP() net.IP {
	return net.IPv4(r.raw[4], r.raw[5], r.raw[6], r.raw[7])
}

func (r Reply) Address() string {
	return fmt.Sprintf("%v:%d", r.IP(), r.Port())
}

func (r *Reply) Serialize() []byte {
	return r.raw
}
