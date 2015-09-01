package watcher_test

import (
	"errors"
	"os"
	"time"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/receptor/fake_receptor"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table/fakes"
	"github.com/cloudfoundry-incubator/tcp-emitter/tcp_routes"
	"github.com/cloudfoundry-incubator/tcp-emitter/watcher"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

type EventHolder struct {
	event receptor.Event
}

var nilEventHolder = EventHolder{}

var _ = Describe("Watcher", func() {

	getDesiredLRP := func(processGuid, logGuid string,
		containerPort, externalPort uint16) receptor.DesiredLRPResponse {
		var desiredLRP receptor.DesiredLRPResponse
		desiredLRP.ProcessGuid = processGuid
		desiredLRP.Ports = []uint16{containerPort}
		desiredLRP.LogGuid = logGuid
		tcpRoutes := tcp_routes.TCPRoutes{
			tcp_routes.TCPRoute{
				ExternalPort:  externalPort,
				ContainerPort: containerPort,
			},
		}
		desiredLRP.Routes = tcpRoutes.RoutingInfo()
		return desiredLRP
	}

	getActualLRP := func(processGuid, instanceGuid, hostAddress string,
		hostPort, containerPort uint16, evacuating bool) receptor.ActualLRPResponse {
		actualLRP := receptor.ActualLRPResponse{
			ProcessGuid:  processGuid,
			InstanceGuid: instanceGuid,
			Address:      hostAddress,
			Evacuating:   evacuating,
			Ports:        []receptor.PortMapping{{ContainerPort: containerPort, HostPort: hostPort}},
		}
		return actualLRP
	}

	var (
		eventSource         *fake_receptor.FakeEventSource
		receptorClient      *fake_receptor.FakeClient
		routingTableHandler *fakes.FakeRoutingTableHandler
		testWatcher         *watcher.Watcher
		clock               *fakeclock.FakeClock
		process             ifrit.Process
		eventChannel        chan receptor.Event
		errorChannel        chan error
		syncChannel         chan struct{}
	)

	BeforeEach(func() {
		eventSource = new(fake_receptor.FakeEventSource)
		receptorClient = new(fake_receptor.FakeClient)
		routingTableHandler = new(fakes.FakeRoutingTableHandler)

		clock = fakeclock.NewFakeClock(time.Now())

		receptorClient.SubscribeToEventsReturns(eventSource, nil)
		syncChannel = make(chan struct{})
		testWatcher = watcher.NewWatcher(receptorClient, clock, routingTableHandler, syncChannel, logger)

		eventChannel = make(chan receptor.Event)
		errorChannel = make(chan error)

		eventSource.CloseStub = func() error {
			errorChannel <- errors.New("closed")
			return nil
		}

		eventSource.NextStub = func() (receptor.Event, error) {
			select {
			case event := <-eventChannel:
				return event, nil
			case err := <-errorChannel:
				return nil, err
			}
			return nil, nil
		}
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(testWatcher)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Context("handle DesiredLRPCreatedEvent", func() {
		var (
			event receptor.Event
		)

		JustBeforeEach(func() {
			desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			event = receptor.NewDesiredLRPCreatedEvent(desiredLRP)
			eventChannel <- event
		})

		It("calls routingTableHandler HandleEvent", func() {
			Eventually(routingTableHandler.HandleEventCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
			createEvent := routingTableHandler.HandleEventArgsForCall(0)
			Expect(createEvent).Should(Equal(event))
		})
	})

	Context("handle DesiredLRPChangedEvent", func() {
		var (
			event receptor.Event
		)

		JustBeforeEach(func() {
			beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61001)
			event = receptor.NewDesiredLRPChangedEvent(beforeLRP, afterLRP)
			eventChannel <- event
		})

		It("calls routingTableHandler HandleEvent", func() {
			Eventually(routingTableHandler.HandleEventCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
			changeEvent := routingTableHandler.HandleEventArgsForCall(0)
			Expect(changeEvent).Should(Equal(event))
		})
	})

	Context("handle DesiredLRPRemovedEvent", func() {
		var (
			event receptor.Event
		)

		JustBeforeEach(func() {
			desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			event = receptor.NewDesiredLRPRemovedEvent(desiredLRP)
			eventChannel <- event
		})

		It("calls routingTableHandler HandleDesiredDelete", func() {
			Eventually(routingTableHandler.HandleEventCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
			deleteEvent := routingTableHandler.HandleEventArgsForCall(0)
			Expect(deleteEvent).Should(Equal(event))
		})
	})

	Context("handle ActualLRPCreatedEvent", func() {
		var (
			event receptor.Event
		)

		JustBeforeEach(func() {
			actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61000, 5222, false)
			event = receptor.NewActualLRPCreatedEvent(actualLRP)
			eventChannel <- event
		})

		It("calls routingTableHandler HandleActualCreate", func() {
			Eventually(routingTableHandler.HandleEventCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
			createEvent := routingTableHandler.HandleEventArgsForCall(0)
			Expect(createEvent).Should(Equal(event))
		})
	})

	Context("handle ActualLRPChangedEvent", func() {
		var (
			event receptor.Event
		)

		JustBeforeEach(func() {
			beforeLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61000, 5222, false)
			afterLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61001, 5222, false)
			event = receptor.NewActualLRPChangedEvent(beforeLRP, afterLRP)
			eventChannel <- event
		})

		It("calls routingTableHandler HandleActualUpdate", func() {
			Eventually(routingTableHandler.HandleEventCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
			changeEvent := routingTableHandler.HandleEventArgsForCall(0)
			Expect(changeEvent).Should(Equal(event))
		})
	})

	Context("handle Sync Event", func() {
		JustBeforeEach(func() {
			syncChannel <- struct{}{}
		})

		It("calls routingTableHandler HandleSync", func() {
			Eventually(routingTableHandler.SyncCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
		})
	})

	Context("when eventSource returns error", func() {
		JustBeforeEach(func() {
			Eventually(receptorClient.SubscribeToEventsCallCount).Should(Equal(1))
			errorChannel <- errors.New("buzinga...")
		})

		It("resubscribes to SSE from receptor", func() {
			Eventually(receptorClient.SubscribeToEventsCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(2))
			Eventually(logger).Should(gbytes.Say("test.watcher.failed-getting-next-event"))
		})
	})

	Context("when subscribe to events fails", func() {
		var (
			receptorErrorChannel chan error
		)
		BeforeEach(func() {
			receptorErrorChannel = make(chan error)

			receptorClient.SubscribeToEventsStub = func() (receptor.EventSource, error) {
				select {
				case err := <-receptorErrorChannel:
					if err != nil {
						return nil, err
					}
				}
				return eventSource, nil
			}

			testWatcher = watcher.NewWatcher(receptorClient, clock, routingTableHandler, syncChannel, logger)
		})

		JustBeforeEach(func() {
			receptorErrorChannel <- errors.New("kaboom")
		})

		It("retries to subscribe", func() {
			close(receptorErrorChannel)
			Eventually(receptorClient.SubscribeToEventsCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(2))
			Eventually(logger).Should(gbytes.Say("test.watcher.failed-subscribing-to-events"))
		})
	})

})
