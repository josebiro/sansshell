package client

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/subcommands"
	"google.golang.org/grpc"

	pb "github.com/snowflakedb/unshelled/services/healthcheck"
)

func init() {
	subcommands.Register(&healthCheckCmd{}, "raw")
}

type healthCheckCmd struct{}

func (*healthCheckCmd) Name() string { return "check" }
func (*healthCheckCmd) Synopsis() string {
	return "send (and print the result of) a simple health check"
}
func (*healthCheckCmd) Usage() string {
	return `check:

  Send a health check to the remote unshelled server, and print the result.
`
}

func (p *healthCheckCmd) SetFlags(f *flag.FlagSet) {}

func (p *healthCheckCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	conn := args[0].(*grpc.ClientConn)

	c := pb.NewHealthCheckClient(conn)

	if _, err := c.Ok(ctx, &pb.Empty{}); err != nil {
		fmt.Fprintf(os.Stderr, "Healthcheck failure: %v\n", err)
	}
	fmt.Println("ok")
	return subcommands.ExitSuccess
}
