package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Snowflake-Labs/sansshell/echo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	hostport   = flag.String("hostport", "localhost:12345", "Host port server is at")
	clientCert = flag.String("client-cert", "./testdata/client.pem", "Path to client cert pem file")
	clientKey  = flag.String("client-key", "./testdata/client.key", "Path to client cert key file")
	rootCA     = flag.String("root-ca", "../testdata/root.pem", "CA file to use for validating client certs")
)

func main() {
	flag.Parse()
	cert, err := tls.LoadX509KeyPair(*clientCert, *clientKey)
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
		Certificates: []tls.Certificate{cert},
		RootCAs:      capool,
		MinVersion:   tls.VersionTLS13,
	})

	ctx := context.Background()
	// Set up a connection to the sansshell-server (possibly via proxy).
	conn, err := grpc.DialContext(ctx, *hostport, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}

	c := echo.NewEchoServiceClient(conn)
	resp, err := c.Echo(ctx, &echo.EchoRequest{Input: "input"})
	if err != nil {
		log.Fatalf("error from Echo: %v", err)
	}
	fmt.Println(resp.Output)
}
