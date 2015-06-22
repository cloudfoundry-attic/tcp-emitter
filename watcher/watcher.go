package watcher

import (
	"os"
	"sync/atomic"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

type Watcher struct {
	receptorClient receptor.Client
	clock          clock.Clock
	logger         lager.Logger
}

func NewWatcher(
	receptorClient receptor.Client,
	clock clock.Clock,
	logger lager.Logger,
) *Watcher {
	return &Watcher{
		receptorClient: receptorClient,
		clock:          clock,
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
		watcher.handleDesiredCreate(logger, event.DesiredLRPResponse)
	case receptor.DesiredLRPChangedEvent:
		watcher.handleDesiredUpdate(logger, event.Before, event.After)
	case receptor.DesiredLRPRemovedEvent:
		watcher.handleDesiredDelete(logger, event.DesiredLRPResponse)
	case receptor.ActualLRPCreatedEvent:
		watcher.handleActualCreate(logger, event.ActualLRPResponse)
	case receptor.ActualLRPChangedEvent:
		watcher.handleActualUpdate(logger, event.Before, event.After)
	case receptor.ActualLRPRemovedEvent:
		watcher.handleActualDelete(logger, event.ActualLRPResponse)
	default:
		logger.Info("did-not-handle-unrecognizable-event", lager.Data{"event-type": event.EventType()})
	}
}

func (watcher *Watcher) handleDesiredCreate(logger lager.Logger, desiredLRP receptor.DesiredLRPResponse) {
	logger = logger.Session("handle-desired-create", desiredLRPData(desiredLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
}

func (watcher *Watcher) handleDesiredUpdate(logger lager.Logger, before, after receptor.DesiredLRPResponse) {
	logger = logger.Session("handling-desired-update", lager.Data{
		"before": desiredLRPData(before),
		"after":  desiredLRPData(after),
	})
	logger.Debug("starting")
	defer logger.Debug("complete")
}

func (watcher *Watcher) handleDesiredDelete(logger lager.Logger, desiredLRP receptor.DesiredLRPResponse) {
	logger = logger.Session("handling-desired-delete", desiredLRPData(desiredLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
}

func (watcher *Watcher) handleActualCreate(logger lager.Logger, actualLRP receptor.ActualLRPResponse) {
	logger = logger.Session("handling-actual-create", actualLRPData(actualLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
}

func (watcher *Watcher) handleActualUpdate(logger lager.Logger, before, after receptor.ActualLRPResponse) {
	logger = logger.Session("handling-actual-update", lager.Data{
		"before": actualLRPData(before),
		"after":  actualLRPData(after),
	})
	logger.Debug("starting")
	defer logger.Debug("complete")

}

func (watcher *Watcher) handleActualDelete(logger lager.Logger, actualLRP receptor.ActualLRPResponse) {
	logger = logger.Session("handling-actual-delete", actualLRPData(actualLRP))
	logger.Debug("starting")
	defer logger.Debug("complete")
}

func desiredLRPData(lrp receptor.DesiredLRPResponse) lager.Data {
	return lager.Data{
		"process-guid": lrp.ProcessGuid,
		"routes":       lrp.Routes,
		"ports":        lrp.Ports,
	}
}

func actualLRPData(lrp receptor.ActualLRPResponse) lager.Data {
	return lager.Data{
		"process-guid":  lrp.ProcessGuid,
		"index":         lrp.Index,
		"domain":        lrp.Domain,
		"instance-guid": lrp.InstanceGuid,
		"cell-id":       lrp.CellID,
		"address":       lrp.Address,
		"ports":         lrp.Ports,
		"evacuating":    lrp.Evacuating,
	}
}
