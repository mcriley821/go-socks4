package proto

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
)

type Request struct {
	//  version byte
	//  command byte
	//  dstPort uint16 BIG
	//  dstAddr uint32 BIG
	//  userID  string
	raw []byte
}

type Command = byte

var (
	InvalidCommand Command = 0
	ConnectCommand Command = 1
	BindCommand    Command = 2
)

const (
	// Maximum allowed size of a socks4 Request.
	// This limits UserID to 63 characters, excluding the null terminator
	maxRequestSize = minRequestSize + 63

	// Minimum possible size of a socks4 Request.
	minRequestSize = 9

	Version = 4
)

func NewRequest(cmd Command, remote string, user string) (*Request, error) {
	if len(user)+minRequestSize > maxRequestSize {
		return nil, errors.New("user must be less than 63 characters")
	}

	host, portStr, err := net.SplitHostPort(remote)
	if err != nil {
		return nil, fmt.Errorf("failed to split remote host & port - %w", err)
	} else if host == "" {
		return nil, errors.New("invalid host")
	} else if portStr == "" {
		return nil, errors.New("invalid port")
	}

	ip := net.ParseIP(host).To4()
	if ip == nil {
		return nil, errors.New("expected a IPv4 remote")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse port as an int - %w", err)
	}

	buff := make([]byte, minRequestSize+len(user))
	buff[0] = Version
	buff[1] = cmd
	binary.BigEndian.PutUint16(buff[2:4], uint16(port))
	copy(buff[4:8], ip)
	copy(buff[8:], user)
	buff[minRequestSize+len(user)-1] = 0

	return &Request{raw: buff}, nil
}

func ReadRequest(conn net.Conn) (*Request, error) {
	rawBytes := make([]byte, maxRequestSize+1)
	n, err := conn.Read(rawBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read from connection - %w", err)
	} else if n < minRequestSize {
		return nil, errors.New("failed to read entire request")
	} else if n > maxRequestSize {
		return nil, errors.New("request is too long")
	}
	return &Request{raw: rawBytes[:n]}, nil
}

func (r Request) Version() int {
	return int(r.raw[0])
}

func (r Request) Command() Command {
	switch r.raw[1] {
	case ConnectCommand:
		return ConnectCommand
	case BindCommand:
		return BindCommand
	default:
		return InvalidCommand
	}
}

func (r Request) Port() int {
	return int(binary.BigEndian.Uint16(r.raw[2:4]))
}

func (r Request) IP() net.IP {
	return net.IPv4(r.raw[4], r.raw[5], r.raw[6], r.raw[7])
}

func (r Request) Address() string {
	return fmt.Sprintf("%v:%d", r.IP(), r.Port())
}

func (r Request) UserID() string {
	return string(r.raw[8 : len(r.raw)-1])
}

func (r Request) Serialize() []byte {
	return r.raw
}
