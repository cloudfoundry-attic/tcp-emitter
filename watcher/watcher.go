package watcher

import (
	"os"
	"sync/atomic"

	"github.com/cloudfoundry-incubator/bbs"
	"github.com/cloudfoundry-incubator/bbs/events"
	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

type Watcher struct {
	bbsClient           bbs.Client
	clock               clock.Clock
	routingTableHandler routing_table.RoutingTableHandler
	syncChannel         chan struct{}
	logger              lager.Logger
}

type syncEndEvent struct {
	table  routing_table.RoutingTable
	logger lager.Logger
}

func NewWatcher(
	bbsClient bbs.Client,
	clock clock.Clock,
	routingTableHandler routing_table.RoutingTableHandler,
	syncChannel chan struct{},
	logger lager.Logger,
) *Watcher {
	return &Watcher{
		bbsClient:           bbsClient,
		clock:               clock,
		routingTableHandler: routingTableHandler,
		syncChannel:         syncChannel,
		logger:              logger.Session("watcher"),
	}
}

func (watcher *Watcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	watcher.logger.Debug("starting")

	close(ready)
	watcher.logger.Debug("started")
	defer watcher.logger.Debug("finished")

	eventChan := make(chan models.Event)

	var eventSource atomic.Value
	var stopEventSource int32

	go func() {
		var err error
		var es events.EventSource

		for {
			if atomic.LoadInt32(&stopEventSource) == 1 {
				return
			}

			watcher.logger.Debug("subscribing-to-bbs-events")
			es, err = watcher.bbsClient.SubscribeToEvents()
			if err != nil {
				watcher.logger.Error("failed-subscribing-to-events", err)
				continue
			}
			watcher.logger.Info("subscribed-to-bbs-events")

			eventSource.Store(es)

			var event models.Event
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
			watcher.routingTableHandler.HandleEvent(event)

		case <-watcher.syncChannel:
			watcher.routingTableHandler.Sync()

		case <-signals:
			watcher.logger.Info("stopping")
			atomic.StoreInt32(&stopEventSource, 1)
			if es := eventSource.Load(); es != nil {
				err := es.(events.EventSource).Close()
				if err != nil {
					watcher.logger.Error("failed-closing-event-source", err)
				}
			}
			return nil
		}
	}
}
