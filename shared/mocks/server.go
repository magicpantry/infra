package mocks

import (
	"context"
	"io"

	"google.golang.org/grpc/metadata"
)

type Server[X, Y any] struct {
	RecvList []X
	SendList []Y

	index int
}

func (s *Server[X, Y]) Reset() {
	s.index = 0
	s.RecvList = []X{}
}

func (s *Server[X, Y]) Send(y Y) error {
	s.SendList = append(s.SendList, y)
	return nil
}

func (s *Server[X, Y]) Recv() (X, error) {
	var x X
	if s.index >= len(s.RecvList) {
		return x, io.EOF
	}
	x = s.RecvList[s.index]
	s.index++
	return x, nil
}

func (s *Server[X, Y]) SendAndClose(y Y) error {
	return nil
}

func (s *Server[X, Y]) SetHeader(md metadata.MD) error {
	return nil
}

func (s *Server[X, Y]) SendHeader(md metadata.MD) error {
	return nil
}

func (s *Server[X, Y]) SetTrailer(md metadata.MD) {}

func (s *Server[X, Y]) Context() context.Context {
	return context.Background()
}

func (s *Server[X, Y]) SendMsg(m any) error {
	return nil
}

func (s *Server[X, Y]) RecvMsg(m any) error {
	return nil
}
