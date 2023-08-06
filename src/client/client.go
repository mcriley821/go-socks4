package client

import (
	"errors"
	"fmt"
	"net"
	"socks4/proto"
)

type Client struct {
	serverAddress string
	user          string
	net.Conn
}

func NewClient(serverAddress string, user string) *Client {
	return &Client{
		serverAddress: serverAddress,
		user:          user,
		Conn:          nil,
	}
}

func (c *Client) connectServer() error {
	conn, err := net.Dial("tcp", c.serverAddress)
	if err != nil {
		return fmt.Errorf("failed to dial server %v - %w", c.serverAddress, err)
	}

	c.Conn = conn
	return nil
}

func (c *Client) makeRequest(remote string, cmd proto.Command) (*proto.Reply, error) {
	if req, err := proto.NewRequest(cmd, remote, c.user); err != nil {
		return nil, fmt.Errorf("failed to create request - %w", err)
	} else if _, err = c.Write(req.Serialize()); err != nil {
		return nil, fmt.Errorf("failed to write request - %w", err)
	}

	return c.readServerReply()
}

func (c *Client) readServerReply() (*proto.Reply, error) {
	resp, err := proto.ReadReply(c.Conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read server reply - %w", err)
	} else if resp.Version() != proto.Version {
		return nil, errors.New("server version does not match client")
	} else if resp.Code() != proto.SuccessReply {
		return nil, errors.New("received error reply from server")
	}

	return resp, nil
}

func (c *Client) Connect(remote string) error {
	if err := c.connectServer(); err != nil {
		return fmt.Errorf("failed to connect to proxy server - %w", err)
	}
	_, err := c.makeRequest(remote, proto.ConnectCommand)
	if err != nil {
		return fmt.Errorf("connect request failed - %w", err)
	}
	return nil
}

func (c *Client) Bind(remote string, onAddressBound func(boundAddress string) error) error {
	if err := c.connectServer(); err != nil {
		return fmt.Errorf("failed to connect to proxy server - %w", err)
	}
	reply, err := c.makeRequest(remote, proto.BindCommand)
	if err != nil {
		return fmt.Errorf("bind request failed - %w", err)
	}

	if err := onAddressBound(reply.Address()); err != nil {
		return fmt.Errorf("onAddressBound call failed - %w", err)
	}

	_, err = c.readServerReply()
	if err != nil {
		return fmt.Errorf("remote failed to connect - %w", err)
	}
	return nil
}

func (c *Client) Write(buff []byte) (int, error) {
	if c.Conn == nil {
		if err := c.connectServer(); err != nil {
			return 0, fmt.Errorf("failed to connect to proxy server - %w", err)
		}
	}

	return c.Conn.Write(buff)
}

func (c *Client) Read(buff []byte) (int, error) {
	if c.Conn == nil {
		if err := c.connectServer(); err != nil {
			return 0, fmt.Errorf("failed to connect to proxy server - %w", err)
		}
	}
	return c.Conn.Read(buff)
}
