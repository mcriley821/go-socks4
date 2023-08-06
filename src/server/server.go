package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"go.uber.org/zap"
)

type Server struct {
	log *zap.Logger
	ln  net.Listener
	wg  sync.WaitGroup
}

func NewServer(log *zap.Logger) *Server {
	return &Server{
		log: log,
		wg:  sync.WaitGroup{},
	}
}

func (s *Server) ListenAndServe(localEndpoint string) (net.Addr, error) {
	var err error

	s.ln, err = net.Listen("tcp", localEndpoint)
	if err != nil {
		s.log.Error("failed to listen", zap.String("endpoint", localEndpoint), zap.Error(err))
		return nil, err
	}

	s.wg.Add(1)
	go s.listenAndServe()
	return s.ln.Addr(), nil
}

func (s *Server) listenAndServe() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				s.log.Error("failed to accept new connection", zap.Error(err))
			}
			break
		}
		go handleNewClient(conn, s.log)
	}
	s.wg.Done()
}

func (s *Server) Close(ctx context.Context) error {
	if s.ln == nil {
		return nil
	}
	if err := s.ln.Close(); err != nil {
		s.log.Error("failed to close listener", zap.Error(err))
		return fmt.Errorf("failed to close listener - %w", err)
	}

	ch := make(chan struct{}, 1)
	go func() {
		s.wg.Wait()
		ch <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != nil {
			s.log.Error("context error closing server", zap.Error(err))
			return err
		}
	case <-ch:
	}
	return nil
}
