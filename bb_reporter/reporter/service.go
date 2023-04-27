package reporter

import (
	"context"
	"fmt"
	"io"
	"time"

	cpb "github.com/buildbarn/bb-remote-execution/pkg/proto/completedactionlogger"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/protobuf/encoding/prototext"
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
		})
	metricCompletedActionCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "bb_reporter",
		Name:      "completed_action_recv_count",
		Help:      "Number of CompletedAction messages received across all streams",
	})
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

type Service struct {
	ctx      context.Context
	recvChan chan *cpb.CompletedAction
	bufChan  chan []*cpb.CompletedAction
}

func NewService(ctx context.Context) (*Service, error) {
	s := &Service{
		ctx:      ctx,
		recvChan: make(chan *cpb.CompletedAction),
		bufChan:  make(chan []*cpb.CompletedAction),
	}

	go s.batchRequestLoop(10, 2*time.Second)
	go s.bigqueryInsertLoop()

	return s, nil
}

func (s *Service) batchRequestLoop(maxBatch int, maxDelay time.Duration) {
	t := time.NewTicker(maxDelay)
	defer t.Stop()

	for {

		buf := make([]*cpb.CompletedAction, 0, maxBatch)

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
				fmt.Println(prototext.Format(reqs[0]))
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

		s.recvChan <- req
		metricCompletedActionCount.Inc()

		empty := &emptypb.Empty{}
		if err := stream.Send(empty); err != nil {
			metricStreamCloseCount.WithLabelValues("send_error").Inc()
			return err
		}
	}
}
