package main_test

import (
	"os"
	"time"

	"github.com/cloudfoundry-incubator/receptor/fake_receptor"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table/fakes"
	"github.com/cloudfoundry-incubator/tcp-emitter/syncer"
	"github.com/cloudfoundry-incubator/tcp-emitter/watcher"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Syncer-Watcher Integration", func() {

	var (
		process             ifrit.Process
		receptorClient      *fake_receptor.FakeClient
		routingTableHandler *fakes.FakeRoutingTableHandler
		clock               *fakeclock.FakeClock
		syncInterval        time.Duration
		logger              lager.Logger
		eventSource         *fake_receptor.FakeEventSource
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		syncInterval = 1 * time.Second

		eventSource = new(fake_receptor.FakeEventSource)
		receptorClient = new(fake_receptor.FakeClient)
		receptorClient.SubscribeToEventsReturns(eventSource, nil)

		routingTableHandler = new(fakes.FakeRoutingTableHandler)
		clock = fakeclock.NewFakeClock(time.Now())
		syncChannel := make(chan struct{})

		syncRunner := syncer.New(clock, syncInterval, syncChannel, logger)
		watcher := watcher.NewWatcher(receptorClient, clock, routingTableHandler, syncChannel, logger)

		members := grouper.Members{
			{"watcher", watcher},
			{"syncer", syncRunner},
		}
		group := grouper.NewOrdered(os.Interrupt, members)

		process = ifrit.Invoke(sigmon.New(group))
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Context("on startup", func() {
		It("watcher invokes sync", func() {
			Eventually(routingTableHandler.SyncCallCount).Should(Equal(1))
		})
	})

	Context("on sync interval", func() {
		It("watcher invokes sync", func() {
			Eventually(routingTableHandler.SyncCallCount).Should(Equal(1))
			clock.Increment(syncInterval + 100*time.Millisecond)
			Eventually(routingTableHandler.SyncCallCount).Should(Equal(2))
		})
	})

})
