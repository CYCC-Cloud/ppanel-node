package cmd

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	pb "github.com/CYCC-Cloud/ppanel-proto/gen/go/ppanel/nodecontrol/v1"
	"github.com/perfect-panel/ppanel-node/api/grpcclient"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const maxControlWatchRetryDelay = 30 * time.Second

type controlWatcher struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

var controlWatchRetryDelay = func(attempt int) time.Duration {
	base := time.Second << min(attempt, 5)
	if base > maxControlWatchRetryDelay {
		base = maxControlWatchRetryDelay
	}
	jitterRange := base / 2
	if base+jitterRange > maxControlWatchRetryDelay {
		jitterRange = maxControlWatchRetryDelay - base
	}
	if jitterRange <= 0 {
		return base
	}
	return base + time.Duration(rand.Int64N(int64(jitterRange)+1))
}

func startControlWatcher(client *grpcclient.Client, req *pb.WatchControlRequest, reloadCh chan<- struct{}) *controlWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	watcher := &controlWatcher{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	request := proto.Clone(req).(*pb.WatchControlRequest)
	go watcher.run(ctx, client, request, reloadCh)
	return watcher
}

func (w *controlWatcher) run(ctx context.Context, client *grpcclient.Client, req *pb.WatchControlRequest, reloadCh chan<- struct{}) {
	defer close(w.done)

	for attempt := 0; ; attempt++ {
		stream, err := client.WatchControl(ctx, req)
		if err == nil {
			err = receiveControlEvents(ctx, stream, reloadCh)
		}
		if ctx.Err() != nil {
			return
		}
		if status.Code(err) == codes.Unimplemented {
			log.Info("WatchControl is not implemented by the server; continuing with polling")
			return
		}

		delay := controlWatchRetryDelay(attempt)
		log.WithError(err).WithField("retry_delay", delay).Warn("WatchControl stream disconnected")
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

type controlEventReceiver interface {
	Recv() (*pb.ControlEvent, error)
}

func receiveControlEvents(ctx context.Context, stream controlEventReceiver, reloadCh chan<- struct{}) error {
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}
		if event.GetConfigChanged() == nil {
			continue
		}
		log.WithField("event_id", event.GetEventId()).Info("Received configuration invalidation")
		select {
		case reloadCh <- struct{}{}:
		default:
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func (w *controlWatcher) Stop() {
	if w == nil {
		return
	}
	w.once.Do(func() {
		w.cancel()
		<-w.done
	})
}
