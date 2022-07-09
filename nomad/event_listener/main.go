package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/enfabrica/enkit/lib/retry"
	"github.com/enfabrica/enkit/lib/server"

	"github.com/golang/glog"
	"github.com/hashicorp/nomad/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	nomadAddr = flag.String("nomad_addr", "", "Nomad API server, in `http://$HOST:$PORT` form")
	// datacenter gets embedded into each metric. Datacenter is not always present
	// in events, but since each event_listener only listens to one datacenter, we
	// can pass in the datacenter statically via this flag.
	datacenter = flag.String("datacenter", "", "Datacenter of Nomad server dialed")
	// startTime keeps track of when this instance was started, so that we can
	// ignore events that occurred before the start time.
	startTime = time.Now().UnixNano()

	metricOoms = promauto.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "nomadevent",
		Name:      "oom_kills",
		Help:      "Count of OOM kill events",
	}, []string{
		"datacenter",
		"job",
		"task_group",
		"task",
	},
	)
)

func main() {
	flag.Parse()

	// Start exporting metrics
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	go server.Run(mux, nil)

	// Connect to Nomad
	client, err := api.NewClient(&api.Config{
		Address: *nomadAddr,
	})
	exitIf(err)

	// If the event stream is disconnected, lastSeenEvent will record the index of
	// the last seen event and gets passed when re-creating the stream, to avoid
	// processing events multiple times.
	lastSeenEvent := uint64(0)
	ctx := context.Background()

	// OOM events turn out to be somewhat obnoxious in Nomad. Instead of there
	// being a Nomad event that corresponds 1:1 with an OOM as one might hope, one
	// only gets "AllocationUpdated" events. Embedded inside these events are an
	// event list per task - these event lists have OOM information, but are sent
	// multiple times in subsequent "AllocationUpdated" Nomad events - counting
	// each one seen will result in many duplicated counts. As a result, we need
	// to deduplicate them; this is done based on the job/group/task and
	// timestamp of the OOM.
	//
	// currentOomState is responsible for the deduplication and counter updating;
	// since it keeps track of all OOMs seen, it needs to be culled periodically
	// or it will consume a strictly-increasing amount of memory.
	currentOomState := newOomState()
	go currentOomState.GarbageCollect()

	err = retry.New(retry.WithWait(10 * time.Second)).Run(func() error {
		stream, err := client.EventStream().Stream(ctx, nil, lastSeenEvent, nil)
		if err != nil {
			return fmt.Errorf("failed to open event stream: %w", err)
		}
		glog.Infof("Listening for events from index %d from %q...", lastSeenEvent, *nomadAddr)
		defer glog.Infof("Stopping listening for events. Last index: %d", lastSeenEvent)

		for events := range stream {
			if events.Err != nil {
				return fmt.Errorf("event stream got error: %w", err)
			}
			exitIf(events.Err)
			lastSeenEvent = events.Index
			for _, event := range events.Events {
				// Per-event processing goes here
				currentOomState.Update(oomEvents(event))
			}
		}

		glog.Warning("Event stream closed unexpectedly")
		return fmt.Errorf("event stream closed unexpectedly")
	})
	exitIf(err)
}

type oomEvent struct {
	Datacenter string
	Job        string
	TaskGroup  string
	Task       string
	Timestamp  int64
}

// oomEvents extracts all the OOM events (if any) from a Nomad event and returns
// the list.
func oomEvents(event api.Event) []oomEvent {
	ooms := []oomEvent{}

	if event.Topic != "Allocation" || event.Type != "AllocationUpdated" {
		return ooms
	}
	alloc, err := event.Allocation()
	if err != nil {
		glog.Warningf("Unable to get Allocation payload from event with type %q", event.Type)
		return ooms
	}
	for task, state := range alloc.TaskStates {
		for _, e := range state.Events {
			if e.Type != "Terminated" {
				continue
			}
			if oomKilled, ok := e.Details["oom_killed"]; ok && oomKilled == "true" {
				ooms = append(ooms, oomEvent{
					Datacenter: *datacenter,
					Job:        alloc.JobID,
					TaskGroup:  alloc.TaskGroup,
					Task:       task,
					Timestamp:  e.Time,
				})
			}
		}
	}
	return ooms
}

type oomState struct {
	mu       sync.Mutex
	oomsSeen map[oomEvent]struct{}
}

func newOomState() *oomState {
	return &oomState{
		oomsSeen: map[oomEvent]struct{}{},
	}
}

// Update counts new oomEvents not seen before, and tracks them to avoid
// counting duplicates.
func (s *oomState) Update(ooms []oomEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, oom := range ooms {
		if _, ok := s.oomsSeen[oom]; !ok && oom.Timestamp > startTime {
			s.oomsSeen[oom] = struct{}{}
			metricOoms.WithLabelValues(oom.Datacenter, oom.Job, oom.TaskGroup, oom.Task).Inc()
		}
	}
}

// GarbageCollect removes very old OOM events from the map on a regular
// interval.
func (s *oomState) GarbageCollect() {
	for {
		time.Sleep(time.Minute)
		s.gcSingleRun()
	}
}

// gcSingleRun removes very old OOM events from the map, by creating a new map
// with only non-stale entries and swapping out the old map.
func (s *oomState) gcSingleRun() {
	s.mu.Lock()
	defer s.mu.Unlock()

	newMap := map[oomEvent]struct{}{}

	threshold := time.Now().Add(-7 * 24 * time.Hour).UnixNano()
	for oom := range s.oomsSeen {
		if oom.Timestamp > threshold {
			newMap[oom] = struct{}{}
		}
	}

	s.oomsSeen = newMap
}

func exitIf(err error) {
	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}
}
