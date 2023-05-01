package reporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	cpb "github.com/buildbarn/bb-remote-execution/pkg/proto/completedactionlogger"
	rupb "github.com/buildbarn/bb-remote-execution/pkg/proto/resourceusage"
	"github.com/golang/glog"
	"github.com/kylelemons/godebug/pretty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	metricActiveStreams = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "bb_reporter",
		Name:      "active_completed_action_stream_count",
		Help:      "Number of currently active CompletedActionLogger streams",
	})
	metricStreamCloseCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bb_reporter",
		Name:      "completed_action_stream_close_count",
		Help:      "Number of CompletedActionLogger stream closes, by reason",
	},
		[]string{
			"reason",
		},
	)
	metricCompletedActionCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bb_reporter",
		Name:      "completed_action_recv_count",
		Help:      "Number of CompletedAction messages received across all streams",
	},
		[]string{
			"parse_outcome",
		},
	)
	metricBatchCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bb_reporter",
		Name:      "completed_action_batch_count",
		Help:      "Number of batches of CompletedActions",
	},
		[]string{
			"trigger",
		},
	)
)

type ActionRecord struct {
	ActionId        string
	ActionDigest    string
	ActionSizeBytes int64
	OutputFiles     []FileRecord

	WorkerCluster      string
	WorkerVirtualNode  string
	WorkerPhysicalNode string
	WorkerThread       uint32

	QueuedTime                time.Time
	WorkerStartTime           time.Time
	WorkerCompletedTime       time.Time
	InputFetchStartTime       time.Time
	InputFetchCompletedTime   time.Time
	ExecutionStartTime        time.Time
	ExecutionCompletedTime    time.Time
	OutputUploadStartTime     time.Time
	OutputUploadCompletedTime time.Time
	VirtualExecutionDuration  time.Duration

	BazelVersion            string
	BazelInvocationId       string
	CorrelatedInvocationsId string
	Mnemonic                string
	Target                  string
	Configuration           string

	UserTime                   time.Duration
	SystemTime                 time.Duration
	MaximumResidentSetSize     int64
	PageReclaims               int64
	PageFaults                 int64
	Swaps                      int64
	BlockInputOperations       int64
	BlockOutputOperations      int64
	VoluntaryContextSwitches   int64
	InvoluntaryContextSwitches int64

	Expenses []Expense
}

type FileRecord struct {
	Filename  string
	SizeBytes int64
}

type Expense struct {
	Name      string
	AmountUSD float64
}

func ActionRecordFromCompletedAction(c *cpb.CompletedAction) (*ActionRecord, error) {
	em := c.GetHistoricalExecuteResponse().GetExecuteResponse().GetResult().GetExecutionMetadata()

	var (
		workerCluster      string
		workerVirtualNode  string
		workerPhysicalNode string
		workerThread       uint32
	)
	workerMeta := map[string]any{}
	if err := json.Unmarshal([]byte(em.GetWorker()), &workerMeta); err != nil {
		glog.Errorf("Failed to unmarshal worker metadata dict on action %q: %v", c.GetUuid(), err)
		return nil, errors.New("worker_metadata_unmarshal_failure")
	}
	if v, ok := workerMeta["nomad_alloc_id"]; ok {
		workerVirtualNode, _ = v.(string)
	}
	if v, ok := workerMeta["nomad_datacenter"]; ok {
		workerCluster, _ = v.(string)
	}
	if v, ok := workerMeta["nomad_node_id"]; ok {
		workerPhysicalNode, _ = v.(string)
	}
	if v, ok := workerMeta["thread"]; ok {
		v, _ := v.(int)
		workerThread = uint32(v)
	}

	var rm *repb.RequestMetadata
	var ru *rupb.POSIXResourceUsage
	var mu *rupb.MonetaryResourceUsage

	for _, any := range em.GetAuxiliaryMetadata() {
		m, err := any.UnmarshalNew()
		if err != nil {
			glog.Errorf("Failed to unmarshal auxiliary_metadata message on action %q: %v", c.GetUuid(), err)
			continue
		}

		switch m := m.(type) {
		case *repb.RequestMetadata:
			rm = m
		case *rupb.POSIXResourceUsage:
			ru = m
		case *rupb.MonetaryResourceUsage:
			mu = m
		case *rupb.FilePoolResourceUsage:
			// No useful info here currently
		default:
			glog.Warningf("Unknown auxiliary_metadata message on action %q: %T", c.GetUuid(), m)
		}
	}

	if rm == nil {
		return nil, errors.New("missing_request_metadata")
	}
	if ru == nil {
		return nil, errors.New("missing_posix_resource_usage")
	}
	if mu == nil {
		return nil, errors.New("missing_cost_info")
	}

	of := c.GetHistoricalExecuteResponse().GetExecuteResponse().GetResult().GetOutputFiles()
	files := make([]FileRecord, 0, len(of))
	for _, file := range of {
		files = append(files, FileRecord{Filename: file.GetPath(), SizeBytes: file.GetDigest().GetSizeBytes()})
	}

	costs := make([]Expense, 0, len(mu.GetExpenses()))
	for name, cost := range mu.GetExpenses() {
		costs = append(costs, Expense{Name: name, AmountUSD: cost.GetCost()})
	}

	a := &ActionRecord{
		ActionId:        c.GetUuid(),
		ActionDigest:    c.GetHistoricalExecuteResponse().GetActionDigest().GetHash(),
		ActionSizeBytes: c.GetHistoricalExecuteResponse().GetActionDigest().GetSizeBytes(),
		OutputFiles:     files,

		WorkerCluster:      workerCluster,
		WorkerVirtualNode:  workerVirtualNode,
		WorkerPhysicalNode: workerPhysicalNode,
		WorkerThread:       workerThread,

		QueuedTime:                em.GetQueuedTimestamp().AsTime(),
		WorkerStartTime:           em.GetWorkerStartTimestamp().AsTime(),
		WorkerCompletedTime:       em.GetWorkerCompletedTimestamp().AsTime(),
		InputFetchStartTime:       em.GetInputFetchStartTimestamp().AsTime(),
		InputFetchCompletedTime:   em.GetInputFetchCompletedTimestamp().AsTime(),
		ExecutionStartTime:        em.GetExecutionStartTimestamp().AsTime(),
		ExecutionCompletedTime:    em.GetExecutionCompletedTimestamp().AsTime(),
		VirtualExecutionDuration:  em.GetVirtualExecutionDuration().AsDuration(),
		OutputUploadStartTime:     em.GetOutputUploadStartTimestamp().AsTime(),
		OutputUploadCompletedTime: em.GetOutputUploadCompletedTimestamp().AsTime(),

		BazelVersion:            rm.GetToolDetails().GetToolVersion(),
		BazelInvocationId:       rm.GetToolInvocationId(),
		CorrelatedInvocationsId: rm.GetCorrelatedInvocationsId(),
		Mnemonic:                rm.GetActionMnemonic(),
		Target:                  rm.GetTargetId(),
		Configuration:           rm.GetConfigurationId(),

		UserTime:                   ru.GetUserTime().AsDuration(),
		SystemTime:                 ru.GetSystemTime().AsDuration(),
		MaximumResidentSetSize:     ru.GetMaximumResidentSetSize(),
		PageReclaims:               ru.GetPageReclaims(),
		PageFaults:                 ru.GetPageFaults(),
		Swaps:                      ru.GetSwaps(),
		BlockInputOperations:       ru.GetBlockInputOperations(),
		BlockOutputOperations:      ru.GetBlockOutputOperations(),
		VoluntaryContextSwitches:   ru.GetVoluntaryContextSwitches(),
		InvoluntaryContextSwitches: ru.GetInvoluntaryContextSwitches(),

		Expenses: costs,
	}

	return a, nil
}

type Service struct {
	ctx      context.Context
	recvChan chan *ActionRecord
	bufChan  chan []*ActionRecord
}

func NewService(ctx context.Context, batchSize int, batchTimeout time.Duration) (*Service, error) {
	s := &Service{
		ctx:      ctx,
		recvChan: make(chan *ActionRecord),
		bufChan:  make(chan []*ActionRecord),
	}

	go s.batchRequestLoop(batchSize, batchTimeout)
	go s.bigqueryInsertLoop()

	return s, nil
}

func (s *Service) batchRequestLoop(maxBatch int, maxDelay time.Duration) {
	t := time.NewTicker(maxDelay)
	defer t.Stop()

	for {

		buf := make([]*ActionRecord, 0, maxBatch)

	batchLoop:
		for {
			select {
			case <-s.ctx.Done():
				glog.Infof("batchRequests got ctx.Done(); exiting")
				return

			case req := <-s.recvChan:
				buf = append(buf, req)
				if len(buf) >= maxBatch {
					metricBatchCount.WithLabelValues("batch_full").Inc()
					break batchLoop
				}

			case <-t.C:
				if len(buf) > 0 {
					metricBatchCount.WithLabelValues("timer_tick").Inc()
					break batchLoop
				}
			}
		}

		s.bufChan <- buf
	}
}

func (s *Service) bigqueryInsertLoop() {
	i := 0

	for {
		select {
		case <-s.ctx.Done():
			glog.Infof("bigqueryInsertLoop got ctx.Done(); exiting")
			return

		case reqs := <-s.bufChan:
			// TODO(scott): Insert into bigquery
			// For now, to get some testdata, log a single message to stdout every 100
			// batches
			i++
			if i%100 == 0 {
				i = 0
				fmt.Println("--------------------------------------------------------------------------------")
				pretty.Print(reqs[0])
			}
		}
	}
}

func (s *Service) LogCompletedActions(stream cpb.CompletedActionLogger_LogCompletedActionsServer) error {
	metricActiveStreams.Inc()
	defer metricActiveStreams.Dec()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			metricStreamCloseCount.WithLabelValues("eof").Inc()
			return nil
		}
		if err != nil {
			metricStreamCloseCount.WithLabelValues("recv_error").Inc()
			return err
		}

		a, err := ActionRecordFromCompletedAction(req)
		if err != nil {
			metricCompletedActionCount.WithLabelValues(err.Error()).Inc()
		} else {
			s.recvChan <- a
			metricCompletedActionCount.WithLabelValues("ok").Inc()
		}

		empty := &emptypb.Empty{}
		if err := stream.Send(empty); err != nil {
			metricStreamCloseCount.WithLabelValues("send_error").Inc()
			return err
		}
	}
}
