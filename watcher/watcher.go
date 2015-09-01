package watcher

import (
	"os"
	"sync/atomic"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

type Watcher struct {
	receptorClient receptor.Client
	clock          clock.Clock
	eventHandler   routing_table.EventHandler
	syncChannel    chan struct{}
	logger         lager.Logger
}

type syncEndEvent struct {
	table  routing_table.RoutingTable
	logger lager.Logger
}

func NewWatcher(
	receptorClient receptor.Client,
	clock clock.Clock,
	eventHandler routing_table.EventHandler,
	syncChannel chan struct{},
	logger lager.Logger,
) *Watcher {
	return &Watcher{
		receptorClient: receptorClient,
		clock:          clock,
		eventHandler:   eventHandler,
		syncChannel:    syncChannel,
		logger:         logger.Session("watcher"),
	}
}

func (watcher *Watcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	watcher.logger.Debug("starting")

	close(ready)
	watcher.logger.Debug("started")
	defer watcher.logger.Debug("finished")

	eventChan := make(chan receptor.Event)

	var eventSource atomic.Value
	var stopEventSource int32

	go func() {
		var err error
		var es receptor.EventSource

		for {
			if atomic.LoadInt32(&stopEventSource) == 1 {
				return
			}

			watcher.logger.Debug("subscribing-to-receptor-events")
			es, err = watcher.receptorClient.SubscribeToEvents()
			if err != nil {
				watcher.logger.Error("failed-subscribing-to-events", err)
				continue
			}
			watcher.logger.Info("subscribed-to-receptor-events")

			eventSource.Store(es)

			var event receptor.Event
			for {
				event, err = es.Next()
				if err != nil {
					watcher.logger.Error("failed-getting-next-event", err)
					break
				}

				if event != nil {
					eventChan <- event
				}
			}
		}
	}()

	for {
		select {
		case event := <-eventChan:
			watcher.eventHandler.HandleEvent(event)

		case <-watcher.syncChannel:
			watcher.eventHandler.Sync()

		case <-signals:
			watcher.logger.Info("stopping")
			atomic.StoreInt32(&stopEventSource, 1)
			if es := eventSource.Load(); es != nil {
				err := es.(receptor.EventSource).Close()
				if err != nil {
					watcher.logger.Error("failed-closing-event-source", err)
				}
			}
			return nil
		}
	}
}
