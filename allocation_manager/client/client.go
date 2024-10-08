package client

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"
	"github.com/enfabrica/enkit/allocation_manager/topology"
)

var runCommand = func(ctx context.Context, result chan error, cmd string, args ...string) {
	job := exec.CommandContext(ctx, cmd, args...)
	job.Stdout = os.Stdout
	job.Stderr = os.Stderr
	result <- job.Run()
}

// AllocationClient wraps a AllocationManagerClient for a specific Unit acquisition.
type AllocationClient struct {
	client        apb.AllocationManagerClient
	invocation    *apb.Invocation // request
	allocated     []*apb.Topology // assigned to us
	allocationErr chan error
}

// New returns a AllocationClient that can be used to guard command invocations
// with the specified Unit.
func New(client apb.AllocationManagerClient, names, configs []string, username, purpose string) *AllocationClient {
	// buildTag string) *AllocationClient {
	a := &AllocationClient{
		client: client,
		invocation: &apb.Invocation{
			Owner:   username,
			Purpose: purpose,
			// BuildTag: buildTag,
		},
		allocationErr: make(chan error),
	}
	inv := a.invocation
	for idx := range names {
		t := apb.Topology{Name: names[idx], Config: configs[idx]}
		inv.Topologies = append(inv.Topologies, &t)
		break // TODO: remove to implement bundles
	}
	return a
}

// Guard wraps the specified command with the Unit allocate/refresh/release
// lifecycle.
func (c *AllocationClient) Guard(ctx context.Context, cmd string, args ...string) error {
	ctx, cancel := context.WithCancel(ctx)
	// Get allocation
	err := c.allocate(ctx)
	if err != nil {
		return fmt.Errorf("failed to allocate Unit: %w", err)
	}
	jobResult := make(chan error)
	go c.refresh(ctx)
	var tempfiles []string
	userTopology, err := topology.LoadYaml(os.Getenv("TESTFRAMEWORK_TOPOLOGY"))
	if err != nil {
		return err
	}
	for _, topo := range c.allocated {
		fh, err := os.CreateTemp("", "topology-*.yaml")
		if err != nil {
			return err
		}
		defer fh.Close()
		// TODO: merge userTopology with c.allocated, before write to temp files
		parsedTopology, err := topology.ParseYaml([]byte(topo.GetConfig()))
		if err != nil {
			return err
		}
		fmt.Printf("parsedTopology: %v\n", parsedTopology)
		mergedTopology := userTopology // plus parsedTopology
		if err != nil {
			return err
		}
		//err = topology.WriteYaml(fh, mergedTopology)
		mergedTopology = mergedTopology // TODO temphack
    fh.Write([]byte(topo.GetConfig()))  // TODO temp hack; spit out same thing
		if err != nil {
			return err
		}
		tempfiles = append(tempfiles, fh.Name())
	}
	// TODO: how to deal with multiple topology files?
	if 1 != len(tempfiles) {
		fmt.Printf("want 1, got %d topologies from server request: %v", len(tempfiles), tempfiles)
	}
	// replace user request with the merged topology files
	os.Setenv("TESTFRAMEWORK_TOPOLOGY", tempfiles[0])
	go runCommand(ctx, jobResult, cmd, args...)
	defer c.release(3 * time.Second)
	select {
	case err := <-c.allocationErr:
		// Allocation lost prematurely
		cancel()
		// Wait for command to fail/be killed
		<-jobResult
		// If the error received was nil, probably the context got cancelled, so
		// report that as the error instead.
		if err == nil {
			err = ctx.Err()
		}
		// No file deletion because something went wrong
		return fmt.Errorf("lost allocation and killed job: %w", err)
	case err := <-jobResult:
		// Command has finished, either with success or error
		if err != nil {
			return fmt.Errorf("job failed: %w", err)
		} else {
			for _, fn := range tempfiles { // delete files if job succeeded
				os.Remove(fn)
			}
		}
		// Stop refreshing
		cancel()
		<-c.allocationErr
		return nil
	}
}

// allocate returns nil if the Unit is successfully allocated, or an error if
// acquisition failed.
func (c *AllocationClient) allocate(ctx context.Context) error {
	var queuePos uint32
	var reqID atomic.Value
	doneChan := make(chan struct{})
	defer close(doneChan)
	go logQueuePosition(&reqID, &queuePos, 30*time.Second, doneChan)
	req := &apb.AllocateRequest{
		Invocation: c.invocation,
	}
	for {
		res, err := c.client.Allocate(ctx, req)
		if err != nil {
			return fmt.Errorf("Allocate() failure: %w", err)
		}
		switch r := res.GetResponseType().(type) {
		case *apb.AllocateResponse_Allocated:
			id := res.GetAllocated().GetId()
			c.allocated = r.Allocated.GetTopologies()
			req.GetInvocation().Id = id
			fmt.Fprintf(os.Stderr, "allocation_manager request: %s\n", id)
			for _, t := range c.allocated {
				fmt.Fprintf(os.Stderr, "allocation_manager reserved Unit: %s\n", t.GetName())
			}
			return nil
		case *apb.AllocateResponse_Queued:
			id := res.GetQueued().GetId()
			req.GetInvocation().Id = id
			reqID.Store(id)
			atomic.StoreUint32(&queuePos, r.Queued.GetQueuePosition())
			sleepTime := min(time.Until(r.Queued.GetNextPollTime().AsTime())*3/5, 5*time.Second)
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
			fmt.Fprintf(os.Stderr, "allocation_manager request %s: queued at position: %v\n", id.Load().(string), atomic.LoadUint32(queuePos))
		case <-done:
			return
		}
	}
}

// refresh refreshes the allocation in a loop until the context is finished.
func (c *AllocationClient) refresh(ctx context.Context) {
	defer close(c.allocationErr)
	for {
		req := &apb.RefreshRequest{
			Invocation: c.invocation,
			Allocated:  c.allocated,
		}
		res, err := c.client.Refresh(ctx, req)
		if err != nil {
			c.allocationErr <- fmt.Errorf("Refresh() failure: %w", err)
		}
		sleepTime := min(time.Until(res.GetRefreshDeadline().AsTime())*3/5, 5*time.Second)
		select {
		case <-time.After(sleepTime):
			continue
		case <-ctx.Done():
			return
		}
	}
}

// release notifies the server that the Unit is no longer required.
func (c *AllocationClient) release(d time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	req := &apb.ReleaseRequest{
		Id: c.invocation.GetId(),
	}
	_, err := c.client.Release(ctx, req)
	if err != nil {
		return fmt.Errorf("error releasing Unit: %w", err)
	}
	return nil
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
