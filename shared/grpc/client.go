package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func Client[T any](host string, fn func(grpc.ClientConnInterface) T) T {
	c, err := ClientErr(host, fn)
	if err != nil {
		log.Fatal(err)
	}
	return c
}

func ClientErr[T any](host string, fn func(grpc.ClientConnInterface) T) (T, error) {
	var out T

	insecure := true
	if !strings.Contains(host, ":") {
		host = fmt.Sprintf("%s:%d", host, 443)
		insecure = false
	}

	var opts []grpc.DialOption
	if host != "" {
		opts = append(opts, grpc.WithAuthority(host))
	}

	if insecure {
		opts = append(opts, grpc.WithInsecure())
	} else {
		systemRoots, err := x509.SystemCertPool()
		if err != nil {
			return out, fmt.Errorf("failed to get system certs: %v", err)
		}
		cred := credentials.NewTLS(&tls.Config{
			RootCAs: systemRoots,
		})
		opts = append(opts, grpc.WithTransportCredentials(cred))
	}

	conn, err := grpc.Dial(host, opts...)
	if err != nil {
		return out, fmt.Errorf("failed to dial '%v': %v", host, err)
	}

	return fn(conn), nil
}
