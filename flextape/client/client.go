package client

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	fpb "github.com/enfabrica/enkit/flextape/proto"
)

var runCommand = func(ctx context.Context, result chan error, cmd string, args ...string) {
	job := exec.CommandContext(ctx, cmd, args...)
	job.Stdout = os.Stdout
	job.Stderr = os.Stderr

	result <- job.Run()
}

// LicenseClient wraps a FlextapeClient for a specific license acquisition.
type LicenseClient struct {
	client     fpb.FlextapeClient
	invocation *fpb.Invocation
	licenseErr chan error
}

// New returns a LicenseClient that can be used to guard command invocations
// with the specified license.
func New(client fpb.FlextapeClient, username string, vendor string, feature string, buildTag string) *LicenseClient {
	return &LicenseClient{
		client: client,
		invocation: &fpb.Invocation{
			Licenses: []*fpb.License{
				&fpb.License{
					Vendor:  vendor,
					Feature: feature,
				},
			},
			Owner:    username,
			BuildTag: buildTag,
		},
		licenseErr: make(chan error),
	}
}

// Guard wraps the specified command with the license acquire/refresh/release
// lifecycle.
func (c *LicenseClient) Guard(ctx context.Context, cmd string, args ...string) error {
	ctx, cancel := context.WithCancel(ctx)
	// Get license
	err := c.acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to obtain license: %w", err)
	}

	jobResult := make(chan error)
	go c.refresh(ctx)
	go runCommand(ctx, jobResult, cmd, args...)

	defer c.release(3 * time.Second)

	select {
	case err := <-c.licenseErr:
		// License lost prematurely
		cancel()
		// Wait for command to fail/be killed
		<-jobResult
		// If the error received was nil, probably the context got cancelled, so
		// report that as the error instead.
		if err == nil {
			err = ctx.Err()
		}
		return fmt.Errorf("lost license and killed job: %w", err)
	case err := <-jobResult:
		// Command has finished, either with success or error
		if err != nil {
			return fmt.Errorf("job failed: %w", err)
		}
		// Stop refreshing
		cancel()
		<-c.licenseErr
		return nil
	}
}

// acquire returns nil if the license is successfully acquired, or an error if
// acquisition failed.
func (c *LicenseClient) acquire(ctx context.Context) error {
	var queuePos uint32
	var reqID atomic.Value
	doneChan := make(chan struct{})
	defer close(doneChan)
	go logQueuePosition(&reqID, &queuePos, 30*time.Second, doneChan)

	req := &fpb.AllocateRequest{
		Invocation: c.invocation,
	}

	for {
		res, err := c.client.Allocate(ctx, req)
		if err != nil {
			return fmt.Errorf("Allocate() failure: %w", err)
		}

		switch r := res.GetResponseType().(type) {
		case *fpb.AllocateResponse_LicenseAllocated:
			req.GetInvocation().Id = r.LicenseAllocated.GetInvocationId()
			fmt.Fprintf(os.Stderr, "flextape request %s: reserved license; running tool\n", r.LicenseAllocated.GetInvocationId())
			return nil
		case *fpb.AllocateResponse_Queued:
			req.GetInvocation().Id = r.Queued.GetInvocationId()
			reqID.Store(req.GetInvocation().GetId())
			atomic.StoreUint32(&queuePos, r.Queued.GetQueuePosition())
			sleepTime := min(time.Until(r.Queued.GetNextPollTime().AsTime())*4/5, time.Second)
			time.Sleep(sleepTime)
			continue
		default:
			return fmt.Errorf("unhandled response type %T", r)
		}
	}
}

// logQueuePosition prints the queue position queuePos to stderr every
// `interval` until `done` is closed.
func logQueuePosition(id *atomic.Value, queuePos *uint32, interval time.Duration, done chan struct{}) {
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			fmt.Fprintf(os.Stderr, "flextape request %s: queued at position: %v\n", id.Load().(string), atomic.LoadUint32(queuePos))
		case <-done:
			return
		}
	}
}

// refresh refreshes the license in a loop until the context is finished.
func (c *LicenseClient) refresh(ctx context.Context) {
	defer close(c.licenseErr)
	for {
		req := &fpb.RefreshRequest{
			Invocation: c.invocation,
		}

		res, err := c.client.Refresh(ctx, req)
		if err != nil {
			c.licenseErr <- fmt.Errorf("Refresh() failure: %w", err)
		}

		sleepTime := min(time.Until(res.GetLicenseRefreshDeadline().AsTime())*4/5, time.Second)

		select {
		case <-time.After(sleepTime):
			continue
		case <-ctx.Done():
			return
		}
	}
}

// release notifies the server that the license is no longer required.
func (c *LicenseClient) release(d time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	req := &fpb.ReleaseRequest{
		InvocationId: c.invocation.GetId(),
	}

	_, err := c.client.Release(ctx, req)
	if err != nil {
		return fmt.Errorf("error releasing license: %w", err)
	}
	return nil
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
