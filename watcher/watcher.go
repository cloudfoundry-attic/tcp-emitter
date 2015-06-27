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
	logger         lager.Logger
}

func NewWatcher(
	receptorClient receptor.Client,
	clock clock.Clock,
	eventHandler routing_table.EventHandler,
	logger lager.Logger,
) *Watcher {
	return &Watcher{
		receptorClient: receptorClient,
		clock:          clock,
		eventHandler:   eventHandler,
		logger:         logger.Session("watcher"),
	}
}

func (watcher *Watcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	watcher.logger.Debug("starting")

	close(ready)
	watcher.logger.Debug("started")
	defer watcher.logger.Debug("finished")

	// var cachedEvents map[string]receptor.Event

	eventChan := make(chan receptor.Event)

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

	// startedEventSource := false
	for {
		select {
		case event := <-eventChan:
			watcher.handleEvent(watcher.logger, event)

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

func (watcher *Watcher) handleEvent(logger lager.Logger, event receptor.Event) {
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
		logger.Info("did-not-handle-unrecognizable-event", lager.Data{"event-type": event.EventType()})
	}
}
