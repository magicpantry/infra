package gcp

import (
	"flag"
	"fmt"
	"log"
	"net"
)

var port = flag.Int("port", 50051, "server port")

func Init() {
	flag.Parse()
}

func Listener() net.Listener {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("listening at: %d", *port)
	return lis
}
