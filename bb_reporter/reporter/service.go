package reporter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	cpb "github.com/buildbarn/bb-remote-execution/pkg/proto/completedactionlogger"
	rupb "github.com/buildbarn/bb-remote-execution/pkg/proto/resourceusage"
	"github.com/golang/glog"
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
	metricRecordInserts = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bb_reporter",
		Name:      "completed_action_insert_count",
		Help:      "Number of records inserted in bigquery, by insert result",
	},
		[]string{
			"outcome",
		},
	)
	metricDeducedFieldCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bb_reporter",
		Name:      "deduced_field_count",
		Help:      "Number of fields that are not explicitly provided, by field name and action",
	},
		[]string{
			"field",  // field that is affected
			"action", // either "omitted" or "defaulted", depending on what the code does
		},
	)
)

type ActionRecord struct {
	ActionId        string       `bigquery:"action_id"`
	ActionDigest    string       `bigquery:"action_digest"`
	ActionSizeBytes int64        `bigquery:"action_size_bytes"`
	OutputFiles     []FileRecord `bigquery:"output_files"`

	WorkerCluster      string `bigquery:"worker_cluster"`
	WorkerVirtualNode  string `bigquery:"worker_virtual_node"`
	WorkerPhysicalNode string `bigquery:"worker_physical_node"`
	WorkerEnvironment  string `bigquery:"worker_environment"`
	WorkerPoolName     string `bigquery:"worker_pool_name"`
	WorkerPoolVersion  string `bigquery:"worker_pool_version"`
	WorkerThread       uint32 `bigquery:"worker_thread"`

	QueuedTime                 time.Time `bigquery:"queued_time"`
	WorkerStartTime            time.Time `bigquery:"worker_start_time"`
	WorkerCompletedTime        time.Time `bigquery:"worker_completed_time"`
	InputFetchStartTime        time.Time `bigquery:"input_fetch_start_time"`
	InputFetchCompletedTime    time.Time `bigquery:"input_fetch_completed_time"`
	ExecutionStartTime         time.Time `bigquery:"execution_start_time"`
	ExecutionCompletedTime     time.Time `bigquery:"execution_completed_time"`
	OutputUploadStartTime      time.Time `bigquery:"output_upload_start_time"`
	OutputUploadCompletedTime  time.Time `bigquery:"output_upload_completed_time"`
	VirtualExecutionDurationNs int64     `bigquery:"virtual_execution_duration_ns"`

	BazelVersion            string `bigquery:"bazel_version"`
	BazelInvocationId       string `bigquery:"bazel_invocation_id"`
	CorrelatedInvocationsId string `bigquery:"correlated_invocations_id"`
	Mnemonic                string `bigquery:"mnemonic"`
	Target                  string `bigquery:"target"`
	Configuration           string `bigquery:"configuration"`

	UserTimeNs                 int64 `bigquery:"user_time_ns"`
	SystemTimeNs               int64 `bigquery:"system_time_ns"`
	MaximumResidentSetSize     int64 `bigquery:"maximum_resident_set_size"`
	PageReclaims               int64 `bigquery:"page_reclaims"`
	PageFaults                 int64 `bigquery:"page_faults"`
	Swaps                      int64 `bigquery:"swaps"`
	BlockInputOperations       int64 `bigquery:"block_input_operations"`
	BlockOutputOperations      int64 `bigquery:"block_output_operations"`
	VoluntaryContextSwitches   int64 `bigquery:"voluntary_context_switches"`
	InvoluntaryContextSwitches int64 `bigquery:"involuntary_context_switches"`

	Expenses []Expense `bigquery:"expenses"`
}

type FileRecord struct {
	Filename  string `bigquery:"filename"`
	SizeBytes int64  `bigquery:"size_bytes"`
}

type Expense struct {
	Name      string  `bigquery:"name"`
	AmountUSD float64 `bigquery:"amount_usd"`
}

func ActionRecordFromCompletedAction(c *cpb.CompletedAction) (*ActionRecord, error) {
	em := c.GetHistoricalExecuteResponse().GetExecuteResponse().GetResult().GetExecutionMetadata()

	var (
		workerCluster      string
		workerVirtualNode  string
		workerPhysicalNode string
		workerEnvironment  string
		workerPoolName     string
		workerPoolVersion  string
		workerThread       uint32
	)
	workerMeta := map[string]any{}
	if err := json.Unmarshal([]byte(em.GetWorker()), &workerMeta); err != nil {
		glog.Errorf("Failed to unmarshal worker metadata dict on action %q: %v", c.GetUuid(), err)
		return nil, errors.New("worker_metadata_unmarshal_failure")
	}

	if v, ok := workerMeta["nomad_alloc_id"]; ok {
		workerVirtualNode, _ = v.(string)
	} else if v, ok := workerMeta["pod"]; ok {
		workerVirtualNode, _ = v.(string)
	} else {
		metricDeducedFieldCount.WithLabelValues("worker_virtual_node", "omitted").Inc()
	}

	if v, ok := workerMeta["nomad_datacenter"]; ok {
		workerCluster, _ = v.(string)
	} else if v, ok := workerMeta["k8s_cluster"]; ok {
		workerCluster, _ = v.(string)
	} else {
		metricDeducedFieldCount.WithLabelValues("worker_cluster", "omitted").Inc()
	}

	if v, ok := workerMeta["nomad_node_id"]; ok {
		workerPhysicalNode, _ = v.(string)
	} else if v, ok := workerMeta["node"]; ok {
		workerPhysicalNode, _ = v.(string)
	} else {
		metricDeducedFieldCount.WithLabelValues("worker_physical_node", "omitted").Inc()
	}

	if v, ok := workerMeta["environment"]; ok {
		workerEnvironment, _ = v.(string)
	} else {
		metricDeducedFieldCount.WithLabelValues("worker_environment", "defaulted").Inc()
		workerEnvironment = "prod"
	}

	if v, ok := workerMeta["pool_name"]; ok {
		workerPoolName, _ = v.(string)
	} else {
		metricDeducedFieldCount.WithLabelValues("worker_pool_name", "defaulted").Inc()
		workerPoolName = "legacy_nomad"
	}

	if v, ok := workerMeta["pool_version"]; ok {
		workerPoolVersion, _ = v.(string)
	} else {
		metricDeducedFieldCount.WithLabelValues("worker_pool_version", "defaulted").Inc()
		workerPoolVersion = "v0"
	}

	if v, ok := workerMeta["thread"]; ok {
		v, _ := v.(int)
		workerThread = uint32(v)
	} else {
		metricDeducedFieldCount.WithLabelValues("worker_thread", "omitted").Inc()
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
		WorkerEnvironment:  workerEnvironment,
		WorkerPoolName:     workerPoolName,
		WorkerPoolVersion:  workerPoolVersion,
		WorkerThread:       workerThread,

		QueuedTime:                 em.GetQueuedTimestamp().AsTime(),
		WorkerStartTime:            em.GetWorkerStartTimestamp().AsTime(),
		WorkerCompletedTime:        em.GetWorkerCompletedTimestamp().AsTime(),
		InputFetchStartTime:        em.GetInputFetchStartTimestamp().AsTime(),
		InputFetchCompletedTime:    em.GetInputFetchCompletedTimestamp().AsTime(),
		ExecutionStartTime:         em.GetExecutionStartTimestamp().AsTime(),
		ExecutionCompletedTime:     em.GetExecutionCompletedTimestamp().AsTime(),
		VirtualExecutionDurationNs: em.GetVirtualExecutionDuration().AsDuration().Nanoseconds(),
		OutputUploadStartTime:      em.GetOutputUploadStartTimestamp().AsTime(),
		OutputUploadCompletedTime:  em.GetOutputUploadCompletedTimestamp().AsTime(),

		BazelVersion:            rm.GetToolDetails().GetToolVersion(),
		BazelInvocationId:       rm.GetToolInvocationId(),
		CorrelatedInvocationsId: rm.GetCorrelatedInvocationsId(),
		Mnemonic:                rm.GetActionMnemonic(),
		Target:                  rm.GetTargetId(),
		Configuration:           rm.GetConfigurationId(),

		UserTimeNs:                 ru.GetUserTime().AsDuration().Nanoseconds(),
		SystemTimeNs:               ru.GetSystemTime().AsDuration().Nanoseconds(),
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
	table    BatchInserter[ActionRecord]
	recvChan chan *ActionRecord
	bufChan  chan []*ActionRecord
}

func NewService(
	ctx context.Context,
	storage BatchInserter[ActionRecord],
	batchSize int,
	batchTimeout time.Duration,
) (*Service, error) {
	s := &Service{
		ctx:      ctx,
		table:    storage,
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
	for {
		select {
		case <-s.ctx.Done():
			glog.Infof("bigqueryInsertLoop got ctx.Done(); exiting")
			return

		case reqs := <-s.bufChan:
			if err := s.table.BatchInsert(s.ctx, reqs); err != nil {
				glog.Errorf("batch insertion of %d ActionRecords failed: %v", len(reqs), err)
				metricRecordInserts.WithLabelValues("insert_failure").Add(float64(len(reqs)))
			} else {
				metricRecordInserts.WithLabelValues("ok").Add(float64(len(reqs)))
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
