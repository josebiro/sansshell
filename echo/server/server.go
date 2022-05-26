package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"log"
	"net"
	"os"

	"github.com/Snowflake-Labs/sansshell/echo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	hostport   = flag.String("hostport", "localhost:12345", "Host port to serve on")
	serverCert = flag.String("server-cert", "./testdata/server.pem", "Path to server cert pem file")
	serverKey  = flag.String("server-key", "./testdata/server.key", "Path to server cert key file")
	rootCA     = flag.String("root-ca", "../testdata/root.pem", "CA file to use for validating client certs")
)

func main() {
	flag.Parse()
	cert, err := tls.LoadX509KeyPair(*serverCert, *serverKey)
	if err != nil {
		log.Fatal(err)
	}

	ca, err := os.ReadFile(*rootCA)
	if err != nil {
		log.Fatal(err)
	}
	capool := x509.NewCertPool()
	if !capool.AppendCertsFromPEM(ca) {
		log.Fatal("can't add CA certs to pool")
	}

	creds := credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    capool,
		MinVersion:   tls.VersionTLS13,
	})
	s := grpc.NewServer(grpc.Creds(creds))
	srv := &echo.Server{}

	srv.Register(s)

	lis, err := net.Listen("tcp", *hostport)
	if err != nil {
		log.Fatal(err)
	}
	s.Serve(lis)
}
