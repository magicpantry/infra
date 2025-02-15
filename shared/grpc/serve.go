package grpc

import (
	"log"
	"net"

	"google.golang.org/grpc"
)

func Serve[T any](lis net.Listener, t T, reg func(grpc.ServiceRegistrar, T)) {
	serv := grpc.NewServer()

	reg(serv, t)

	log.Printf("%v", serv.Serve(lis))
}
