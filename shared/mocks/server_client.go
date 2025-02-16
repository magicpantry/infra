package mocks

import (
	"context"
	"errors"
	"io"

	"google.golang.org/grpc/metadata"
)

type ServerClient[X, Y any] struct {
	ReturnErr bool

	RecvList []Y
	SendList []X

	index int
}

func (s *ServerClient[X, Y]) Reset() {
	s.index = 0
	s.RecvList = []Y{}
}

func (s *ServerClient[X, Y]) Header() (metadata.MD, error) {
	return nil, nil
}

func (s *ServerClient[X, Y]) Trailer() metadata.MD {
	return nil
}

func (s *ServerClient[X, Y]) CloseSend() error {
	return nil
}

func (s *ServerClient[X, Y]) Context() context.Context {
	return context.Background()
}

func (s *ServerClient[X, Y]) SendMsg(m any) error {
	return nil
}

func (s *ServerClient[X, Y]) RecvMsg(m any) error {
	return nil
}

func (s *ServerClient[X, Y]) Recv() (Y, error) {
	var y Y
	if s.ReturnErr {
		return y, errors.New("error")
	}
	if s.index >= len(s.RecvList) {
		return y, io.EOF
	}
	y = s.RecvList[s.index]
	s.index++
	return y, nil
}

func (s *ServerClient[X, Y]) Send(x X) error {
	if s.ReturnErr {
		return errors.New("error")
	}
	s.SendList = append(s.SendList, x)
	return nil
}

func (s *ServerClient[X, Y]) CloseAndRecv() (Y, error) {
	var y Y
	if s.ReturnErr {
		return y, errors.New("error")
	}
	return y, nil
}
