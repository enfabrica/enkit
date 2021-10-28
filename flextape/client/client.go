package client

import (
	"fmt"
	"time"

	"google.golang.org/grpc"
)

func RunCommandWithLicense(conn *grpc.ClientConn, username string, vendor string, feature string, timeout time.Duration, cmd string, args ...string) error {
	return fmt.Errorf("flextape client not yet implemented")
}