#!/bin/sh

cd client
go build .
cd ../server
go build .

GRPC_GO_LOG_SEVERITY_LEVEL=info
GRPC_GO_LOG_VERBOSITY_LEVEL=99
export GRPC_GO_LOG_SEVERITY_LEVEL GRPC_GO_LOG_VERBOSITY_LEVEL

./server >server.log 2>&1 &
PID=$!
trap "kill -9 ${PID}" EXIT INT TERM HUP

unset GRPC_GO_LOG_SEVERITY_LEVEL
unset GRPC_GO_LOG_VERBOSITY_LEVEL

cd ../client
echo "Client with cert server knows (CA)"
./client
echo

GRPC_GO_LOG_SEVERITY_LEVEL=info
GRPC_GO_LOG_VERBOSITY_LEVEL=99
export GRPC_GO_LOG_SEVERITY_LEVEL GRPC_GO_LOG_VERBOSITY_LEVEL
echo "Client with cert server doesn't know (CA)"
./client --client-key=./testdata/bad.key --client-cert=./testdata/bad.pem
echo

echo "Client with wrong CA for server"
./client --root-ca=./testdata/bad-root.pem
echo
