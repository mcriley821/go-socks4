package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"socks4/proto"
	"strconv"
	"time"

	"go.uber.org/zap"
)

func handleNewClient(conn net.Conn, log *zap.Logger) {
	log = log.With(zap.String("client", conn.RemoteAddr().String()))
	log.Info("handling new client")

	deadline := time.Now().Add(time.Minute * 2)
	conn.SetDeadline(deadline)
	defer conn.Close()

	req, err := proto.ReadRequest(conn)
	if err != nil {
		log.Error("failed to read request", zap.Error(err))
		return
	} else if req.Version() != proto.Version {
		log.Error("not a socks4 request")
		return
	}

	remote, err := handleRequest(conn, deadline, req)
	if err != nil {
		log.Error("failed to handle request", zap.Error(err))
		err := sendReply(conn, proto.ErrorReply, req.IP(), req.Port())
		if err != nil {
			log.Error("failed to send error response", zap.Error(err))
		}
		return
	}
	defer remote.Close()

	err = sendReply(conn, proto.SuccessReply, req.IP(), req.Port())
	if err != nil {
		log.Error("failed to send success response", zap.Error(err))
		return
	}

	if err := exchangePump(conn, remote); err != nil {
		log.Error("exchange pump failure", zap.Error(err))
		return
	}

	log.Info("client disconnected")
}

func handleRequest(conn net.Conn, deadline time.Time, req *proto.Request) (net.Conn, error) {
	switch req.Command() {
	case proto.ConnectCommand:
		return doConnect(conn, deadline, req)
	case proto.BindCommand:
		return doBind(conn, deadline, req)
	default:
		return nil, errors.New("invalid request command")
	}
}

func doConnect(conn net.Conn, deadline time.Time, req *proto.Request) (net.Conn, error) {
	d := net.Dialer{}
	ctx, cancel := context.WithDeadline(context.Background(), deadline.Add(-time.Second))
	remote, err := d.DialContext(ctx, "tcp", req.Address())
	cancel()
	if err != nil {
		return nil, fmt.Errorf("failed to dial requested address - %w", err)
	}
	return remote, nil
}

func doBind(conn net.Conn, deadline time.Time, req *proto.Request) (net.Conn, error) {
	ln, err := net.ListenTCP("tcp4", &net.TCPAddr{})
	if err != nil {
		return nil, fmt.Errorf("failed to listen - %w", err)
	}
	defer ln.Close()

	if err := ln.SetDeadline(deadline.Add(-time.Second)); err != nil {
		return nil, fmt.Errorf("failed to set listener deadline - %w", err)
	}

	var lnPort int
	if _, port, err := net.SplitHostPort(ln.Addr().String()); err != nil {
		return nil, fmt.Errorf("failed to get listener port - %w", err)
	} else if val, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("failed to parse listener port - %w", err)
	} else {
		lnPort = val
	}

	err = sendReply(conn, proto.SuccessReply, net.IPv4(0, 0, 0, 0), lnPort)
	if err != nil {
		return nil, fmt.Errorf("failed to send initial bind success - %w", err)
	}

	remote, err := ln.Accept()
	if err != nil {
		return nil, fmt.Errorf("failed to accept remote - %w", err)
	}

	host, _, err := net.SplitHostPort(remote.RemoteAddr().String())
	if err != nil {
		return nil, fmt.Errorf("failed to split host from remote addr - %w", err)
	}

	if host != req.IP().String() {
		remote.Close()
		return nil, errors.New("requested remote does not match connected remote")
	}

	return remote, nil
}

func sendReply(conn net.Conn, code proto.ReplyCode, ip net.IP, port int) error {
	body := proto.NewReply(code, ip, port).Serialize()
	n, err := conn.Write(body)
	if err != nil {
		return fmt.Errorf("failed to write to client - %w", err)
	} else if n != len(body) {
		return errors.New("failed to write entire payload to client")
	}
	return nil
}

func exchangePump(client, remote net.Conn) error {
	errChan := make(chan error, 1)

	// net.Conns are concurrent-safe
	go exchange(client, remote, errChan)
	go exchange(remote, client, errChan)

	err := <-errChan
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func exchange(reader, writer net.Conn, errChan chan<- error) {
	buffer := make([]byte, 1<<16)
	for {
		if err := setDeadlines(reader, writer); err != nil {
			errChan <- err
			return
		}
		n, err := reader.Read(buffer)
		if err != nil {
			errChan <- err
			return
		}
		_, err = writer.Write(buffer[:n])
		if err != nil {
			errChan <- err
			return
		}
	}
}

func setDeadlines(reader, writer net.Conn) error {
	deadline := time.Now().Add(time.Second * 30)
	if err := reader.SetReadDeadline(deadline); err != nil {
		return err
	}
	return writer.SetWriteDeadline(deadline)
}
