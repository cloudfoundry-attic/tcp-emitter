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
		eventSource    *fake_receptor.FakeEventSource
		receptorClient *fake_receptor.FakeClient
		eventHandler   *fakes.FakeEventHandler
		testWatcher    *watcher.Watcher
		clock          *fakeclock.FakeClock
		process        ifrit.Process
		eventChannel   chan receptor.Event
		errorChannel   chan error
		syncChannel    chan struct{}
	)

	BeforeEach(func() {
		eventSource = new(fake_receptor.FakeEventSource)
		receptorClient = new(fake_receptor.FakeClient)
		eventHandler = new(fakes.FakeEventHandler)

		clock = fakeclock.NewFakeClock(time.Now())

		receptorClient.SubscribeToEventsReturns(eventSource, nil)
		syncChannel = make(chan struct{})
		testWatcher = watcher.NewWatcher(receptorClient, clock, eventHandler, syncChannel, logger)

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
			desiredLRP receptor.DesiredLRPResponse
		)

		JustBeforeEach(func() {
			desiredLRP = getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			eventChannel <- receptor.NewDesiredLRPCreatedEvent(desiredLRP)
		})

		It("calls eventHandler HandleDesiredCreate", func() {
			Eventually(eventHandler.HandleDesiredCreateCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
			lrp := eventHandler.HandleDesiredCreateArgsForCall(0)
			Expect(lrp).Should(Equal(desiredLRP))
		})
	})

	Context("handle DesiredLRPChangedEvent", func() {
		var (
			beforeLRP receptor.DesiredLRPResponse
			afterLRP  receptor.DesiredLRPResponse
		)

		JustBeforeEach(func() {
			beforeLRP = getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			afterLRP = getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61001)
			eventChannel <- receptor.NewDesiredLRPChangedEvent(beforeLRP, afterLRP)
		})

		It("calls eventHandler HandleDesiredUpdate", func() {
			Eventually(eventHandler.HandleDesiredUpdateCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
			before, after := eventHandler.HandleDesiredUpdateArgsForCall(0)
			Expect(before).Should(Equal(beforeLRP))
			Expect(after).Should(Equal(afterLRP))
		})
	})

	Context("handle DesiredLRPRemovedEvent", func() {
		var (
			desiredLRP receptor.DesiredLRPResponse
		)

		JustBeforeEach(func() {
			desiredLRP = getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			eventChannel <- receptor.NewDesiredLRPRemovedEvent(desiredLRP)
		})

		It("calls eventHandler HandleDesiredDelete", func() {
			Eventually(eventHandler.HandleDesiredDeleteCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
			lrp := eventHandler.HandleDesiredDeleteArgsForCall(0)
			Expect(lrp).Should(Equal(desiredLRP))
		})
	})

	Context("handle ActualLRPCreatedEvent", func() {
		var (
			actualLRP receptor.ActualLRPResponse
		)

		JustBeforeEach(func() {
			actualLRP = getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61000, 5222, false)
			eventChannel <- receptor.NewActualLRPCreatedEvent(actualLRP)
		})

		It("calls eventHandler HandleActualCreate", func() {
			Eventually(eventHandler.HandleActualCreateCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
			lrp := eventHandler.HandleActualCreateArgsForCall(0)
			Expect(lrp).Should(Equal(actualLRP))
		})
	})

	Context("handle ActualLRPChangedEvent", func() {
		var (
			beforeLRP receptor.ActualLRPResponse
			afterLRP  receptor.ActualLRPResponse
		)

		JustBeforeEach(func() {
			beforeLRP = getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61000, 5222, false)
			afterLRP = getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61001, 5222, false)
			eventChannel <- receptor.NewActualLRPChangedEvent(beforeLRP, afterLRP)
		})

		It("calls eventHandler HandleActualUpdate", func() {
			Eventually(eventHandler.HandleActualUpdateCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
			before, after := eventHandler.HandleActualUpdateArgsForCall(0)
			Expect(before).Should(Equal(beforeLRP))
			Expect(after).Should(Equal(afterLRP))
		})
	})

	Context("handle ActualLRPRemovedEvent", func() {
		var (
			actualLRP receptor.ActualLRPResponse
		)

		JustBeforeEach(func() {
			actualLRP = getActualLRP("process-guid-1", "instance-guid-1", "some-ip", 61000, 5222, false)
			eventChannel <- receptor.NewActualLRPRemovedEvent(actualLRP)
		})

		It("calls eventHandler HandleActualDelete", func() {
			Eventually(eventHandler.HandleActualDeleteCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
			lrp := eventHandler.HandleActualDeleteArgsForCall(0)
			Expect(lrp).Should(Equal(actualLRP))
		})
	})

	Context("handle Sync Event", func() {
		JustBeforeEach(func() {
			syncChannel <- struct{}{}
		})

		It("calls eventHandler HandleSync", func() {
			Eventually(eventHandler.HandleSyncCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(1))
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

			testWatcher = watcher.NewWatcher(receptorClient, clock, eventHandler, syncChannel, logger)
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
