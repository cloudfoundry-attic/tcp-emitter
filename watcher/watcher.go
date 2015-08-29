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

	var cachedEvents []receptor.Event

	eventChan := make(chan receptor.Event)
	syncEndChan := make(chan syncEndEvent)

	var eventSource atomic.Value
	var stopEventSource int32

	startEventSource := func() {
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
	}

	startEventSource()

	syncing := false
	// startedEventSource := false
	for {
		select {
		case event := <-eventChan:
			if syncing {
				watcher.logger.Info("caching-event", lager.Data{
					"type": event.EventType(),
				})
				cachedEvents = append(cachedEvents, event)
			} else {
				watcher.handleEvent(event)
			}

		case <-watcher.syncChannel:
			if syncing == false {
				syncing = true

				logger := watcher.logger.Session("sync")
				logger.Info("starting")

				cachedEvents = []receptor.Event{}
				go watcher.handleSync(logger, syncEndChan)
			}

		case syncEnd := <-syncEndChan:
			watcher.completeSync(syncEnd, cachedEvents)
			// is this enough for GC ?
			cachedEvents = nil
			syncing = false
			syncEnd.logger.Info("complete")

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

func (watcher *Watcher) handleSync(logger lager.Logger, syncEndChan chan syncEndEvent) {
	endEvent := syncEndEvent{logger: logger}
	defer func() {
		syncEndChan <- endEvent
	}()
	syncRoutingTable := watcher.eventHandler.HandleSync()
	endEvent.table = syncRoutingTable
}

func (watcher *Watcher) handleEvent(event receptor.Event) {
	switch event := event.(type) {
	case receptor.DesiredLRPCreatedEvent:
		watcher.eventHandler.HandleDesiredCreate(event.DesiredLRPResponse)
	case receptor.DesiredLRPChangedEvent:
		watcher.eventHandler.HandleDesiredUpdate(event.Before, event.After)
	case receptor.DesiredLRPRemovedEvent:
		watcher.eventHandler.HandleDesiredDelete(event.DesiredLRPResponse)
	case receptor.ActualLRPCreatedEvent:
		watcher.eventHandler.HandleActualCreate(event.ActualLRPResponse)
	case receptor.ActualLRPChangedEvent:
		watcher.eventHandler.HandleActualUpdate(event.Before, event.After)
	case receptor.ActualLRPRemovedEvent:
		watcher.eventHandler.HandleActualDelete(event.ActualLRPResponse)
	default:
		watcher.logger.Info("did-not-handle-unrecognizable-event", lager.Data{"event-type": event.EventType()})
	}
}

func (watcher *Watcher) completeSync(syncEnd syncEndEvent, cachedEvents []receptor.Event) {

}
