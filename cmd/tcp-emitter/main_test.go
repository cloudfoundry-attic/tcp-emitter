package main_test

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/bbs/events/eventfakes"
	"github.com/cloudfoundry-incubator/bbs/fake_bbs"
	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table/fakes"
	"github.com/cloudfoundry-incubator/tcp-emitter/syncer"
	"github.com/cloudfoundry-incubator/tcp-emitter/tcp_routes"
	"github.com/cloudfoundry-incubator/tcp-emitter/watcher"
	"github.com/gogo/protobuf/proto"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	routingtestrunner "github.com/cloudfoundry-incubator/routing-api/cmd/routing-api/testrunner"
	"github.com/cloudfoundry-incubator/routing-api/db"
	"github.com/cloudfoundry-incubator/tcp-emitter/cmd/tcp-emitter/testrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("TCP Emitter", func() {
	Describe("Syncer-Watcher Integration", func() {
		var (
			process             ifrit.Process
			bbsClient           *fake_bbs.FakeClient
			routingTableHandler *fakes.FakeRoutingTableHandler
			clock               *fakeclock.FakeClock
			syncInterval        time.Duration
			logger              lager.Logger
			eventSource         *eventfakes.FakeEventSource
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			syncInterval = 1 * time.Second

			eventSource = new(eventfakes.FakeEventSource)
			bbsClient = new(fake_bbs.FakeClient)
			bbsClient.SubscribeToEventsReturns(eventSource, nil)

			routingTableHandler = new(fakes.FakeRoutingTableHandler)
			clock = fakeclock.NewFakeClock(time.Now())
			syncChannel := make(chan struct{})

			syncRunner := syncer.New(clock, syncInterval, syncChannel, logger)
			watcher := watcher.NewWatcher(bbsClient, clock, routingTableHandler, syncChannel, logger)

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

	Describe("Main", func() {
		setupBbsServer := func(server *ghttp.Server) {
			server.RouteToHandler("POST", "/v1/actual_lrp_groups/list",
				func(w http.ResponseWriter, req *http.Request) {
					actualLRPs := []*models.ActualLRPGroup{
						&models.ActualLRPGroup{
							Instance: &models.ActualLRP{
								ActualLRPKey:         models.NewActualLRPKey("some-guid", 0, "domain"),
								ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id-1"),
								ActualLRPNetInfo: models.NewActualLRPNetInfo(
									"some-ip",
									models.NewPortMapping(62003, 5222),
								),
								State: models.ActualLRPStateRunning,
							},
							Evacuating: nil,
						},
					}
					actualLRPResponse := models.ActualLRPGroupsResponse{
						ActualLrpGroups: actualLRPs,
					}
					data, _ := proto.Marshal(&actualLRPResponse)
					w.Header().Set("Content-Length", strconv.Itoa(len(data)))
					w.Header().Set("Content-Type", "application/x-protobuf")
					w.WriteHeader(http.StatusOK)
					w.Write(data)
				})
			server.RouteToHandler("POST", "/v1/desired_lrps/list",
				func(w http.ResponseWriter, req *http.Request) {
					var desiredLRP models.DesiredLRP
					desiredLRP.ProcessGuid = "some-guid"
					desiredLRP.Ports = []uint32{5222}
					desiredLRP.LogGuid = "log-guid"
					tcpRoutes := tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							ExternalPort:  5222,
							ContainerPort: 5222,
						},
					}
					desiredLRP.Routes = tcpRoutes.RoutingInfo()
					desiredLRPs := []*models.DesiredLRP{
						&desiredLRP,
					}
					desiredLRPResponse := models.DesiredLRPsResponse{
						DesiredLrps: desiredLRPs,
					}
					data, _ := proto.Marshal(&desiredLRPResponse)
					w.Header().Set("Content-Length", strconv.Itoa(len(data)))
					w.Header().Set("Content-Type", "application/x-protobuf")
					w.WriteHeader(http.StatusOK)
					w.Write(data)
				})
			server.RouteToHandler("GET", "/v1/events",
				func(w http.ResponseWriter, req *http.Request) {
				})
		}

		setupRoutingApiServer := func(path string, args routingtestrunner.Args) ifrit.Process {
			routingApiServer := routingtestrunner.New(path, args)
			return ifrit.Invoke(routingApiServer)
		}

		setupTcpEmitter := func(path string, args testrunner.Args, expectStarted bool) *gexec.Session {
			allOutput := gbytes.NewBuffer()
			runner := testrunner.New(path, args)
			session, err := gexec.Start(runner.Command, allOutput, allOutput)
			Expect(err).ToNot(HaveOccurred())
			if expectStarted {
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("tcp-emitter.started"))
			} else {
				Consistently(session.Out, 5*time.Second).ShouldNot(gbytes.Say("tcp-emitter.started"))
			}
			return session
		}

		eventsEndpointRequests := func() int {
			requests := make([]*http.Request, 0)
			receivedRequests := bbsServer.ReceivedRequests()
			for _, req := range receivedRequests {
				if strings.Contains(req.RequestURI, "/v1/events") {
					requests = append(requests, req)
				}
			}
			return len(requests)
		}

		checkEmitterWorks := func(session *gexec.Session) {
			Eventually(eventsEndpointRequests, 5*time.Second).Should(BeNumerically(">=", 1))
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("subscribed-to-bbs-event"))
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("syncer.syncing"))
			Consistently(session.Out, 5*time.Second).ShouldNot(gbytes.Say("unable-to-upsert"))
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("successfully-emitted-events"))
		}

		checkTcpRouteMapping := func(tcpRouteMapping db.TcpRouteMapping) {
			tcpRouteMappings, err := routingApiClient.TcpRouteMappings()
			Expect(err).NotTo(HaveOccurred())
			Expect(tcpRouteMappings).To(ContainElement(tcpRouteMapping))
		}

		var (
			expectedTcpRouteMapping db.TcpRouteMapping
		)

		BeforeEach(func() {
			expectedTcpRouteMapping = db.TcpRouteMapping{
				TcpRoute: db.TcpRoute{
					RouterGroupGuid: routing_table.DefaultRouterGroupGuid,
					ExternalPort:    5222,
				},
				HostPort: 62003,
				HostIP:   "some-ip",
			}
		})

		Context("when invalid bbs address is passed to tcp emitter", func() {
			var (
				session *gexec.Session
			)

			BeforeEach(func() {
				invalidTcpEmitterArgs := testrunner.Args{
					BBSAddress:     "127.0.0.1",
					BBSClientCert:  "",
					BBSCACert:      "",
					BBSClientKey:   "",
					ConfigFilePath: createEmitterConfig(),
					SyncInterval:   1 * time.Second,
					ConsulCluster:  consulRunner.ConsulCluster(),
				}
				session = setupTcpEmitter(tcpEmitterBinPath, invalidTcpEmitterArgs, false)
			})

			It("fails to come up", func() {
				Eventually(session.Exited, 5*time.Second).Should(BeClosed())
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("invalid-scheme-in-bbs-address"))
			})
		})

		Context("when protocol is http", func() {
			var (
				session *gexec.Session
			)

			BeforeEach(func() {
				tcpEmitterArgs := testrunner.Args{
					BBSAddress:     bbsServer.URL(),
					BBSClientCert:  "",
					BBSCACert:      "",
					BBSClientKey:   "",
					ConfigFilePath: createEmitterConfig(),
					SyncInterval:   1 * time.Second,
					ConsulCluster:  consulRunner.ConsulCluster(),
				}
				session = setupTcpEmitter(tcpEmitterBinPath, tcpEmitterArgs, true)
			})

			AfterEach(func() {
				logger.Info("shutting-down")
				session.Signal(os.Interrupt)
				Eventually(session.Exited, 5*time.Second).Should(BeClosed())
			})

			It("does not use the secure bbs client", func() {
				Consistently(session.Out, 5*time.Second).ShouldNot(gbytes.Say("setting-up-secure-bbs-client"))
			})
		})

		Context("when both bbs and routing api server are up and running", func() {
			var (
				routingApiProcess ifrit.Process
				session           *gexec.Session
			)
			BeforeEach(func() {
				setupBbsServer(bbsServer)
				routingApiProcess = setupRoutingApiServer(routingAPIBinPath, routingAPIArgs)
				logger.Info("started-routing-api-server")
				session = setupTcpEmitter(tcpEmitterBinPath, tcpEmitterArgs, true)
				logger.Info("started-tcp-emitter")
			})

			AfterEach(func() {
				logger.Info("shutting-down")
				session.Signal(os.Interrupt)
				Eventually(session.Exited, 5*time.Second).Should(BeClosed())
				routingApiProcess.Signal(os.Interrupt)
				Eventually(routingApiProcess.Wait(), 5*time.Second).Should(Receive())
			})

			It("starts an SSE connection to the bbs and emits events to routing api", func() {
				checkEmitterWorks(session)
				checkTcpRouteMapping(expectedTcpRouteMapping)
			})
		})

		Context("when routing api server is down but bbs is running", func() {
			var (
				routingApiProcess ifrit.Process
				session           *gexec.Session
			)

			BeforeEach(func() {
				setupBbsServer(bbsServer)
				session = setupTcpEmitter(tcpEmitterBinPath, tcpEmitterArgs, true)
				logger.Info("started-tcp-emitter")
			})

			AfterEach(func() {
				logger.Info("shutting-down")
				session.Signal(os.Interrupt)
				Eventually(session.Exited, 5*time.Second).Should(BeClosed())
				routingApiProcess.Signal(os.Interrupt)
				Eventually(routingApiProcess.Wait(), 5*time.Second).Should(Receive())
			})

			It("starts an SSE connection to the bbs and continues to try to emit to routing api", func() {
				Eventually(eventsEndpointRequests, 5*time.Second).Should(BeNumerically(">=", 1))

				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("subscribed-to-bbs-event"))
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("syncer.syncing"))
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("unable-to-upsert"))
				Consistently(session.Out, 5*time.Second).ShouldNot(gbytes.Say("successfully-emitted-event"))
				Consistently(session.Exited).ShouldNot(BeClosed())

				By("starting routing api server")
				routingApiProcess = setupRoutingApiServer(routingAPIBinPath, routingAPIArgs)
				logger.Info("started-routing-api-server")
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("successfully-emitted-event"))
			})

		})

		Context("when bbs server is down but routing api is running", func() {
			var (
				routingApiProcess ifrit.Process
				session           *gexec.Session
			)

			BeforeEach(func() {
				routingApiProcess = setupRoutingApiServer(routingAPIBinPath, routingAPIArgs)
				logger.Info("started-routing-api-server")
				bbsServer.Close()
				session = setupTcpEmitter(tcpEmitterBinPath, tcpEmitterArgs, true)
				logger.Info("started-tcp-emitter")
			})

			AfterEach(func() {
				logger.Info("shutting-down")
				session.Signal(os.Interrupt)
				Eventually(session.Exited, 5*time.Second).Should(BeClosed())
				routingApiProcess.Signal(os.Interrupt)
				Eventually(routingApiProcess.Wait(), 5*time.Second).Should(Receive())
			})

			It("tries to start an SSE connection to the bbs and doesn't blow up", func() {
				Consistently(session.Out, 5*time.Second).ShouldNot(gbytes.Say("failed-subscribing-to-events"))
				Consistently(session.Exited).ShouldNot(BeClosed())
				bbsServer = ghttp.NewServer()
			})
		})

		Context("when both bbs and routing api server are up and running", func() {
			var (
				routingApiProcess ifrit.Process
				session1          *gexec.Session
			)
			BeforeEach(func() {
				setupBbsServer(bbsServer)
				routingApiProcess = setupRoutingApiServer(routingAPIBinPath, routingAPIArgs)
				logger.Info("started-routing-api-server")
				session1 = setupTcpEmitter(tcpEmitterBinPath, tcpEmitterArgs, true)
				logger.Info("started-tcp-emitter")
			})

			AfterEach(func() {
				logger.Info("shutting-down")
				session1.Signal(os.Interrupt)
				Eventually(session1.Exited, 5*time.Second).Should(BeClosed())
				routingApiProcess.Signal(os.Interrupt)
				Eventually(routingApiProcess.Wait(), 5*time.Second).Should(Receive())
			})

			It("and the first emitter starts an SSE connection to the bbs and emits events to routing api", func() {
				checkEmitterWorks(session1)
				checkTcpRouteMapping(expectedTcpRouteMapping)
			})

			Context("and another emitter starts", func() {
				var (
					session2 *gexec.Session
				)

				BeforeEach(func() {
					tcpEmitterArgs.SessionName = "tcp-emitter-2"
					session2 = setupTcpEmitter(tcpEmitterBinPath, tcpEmitterArgs, false)
					logger.Info("started-tcp-emitter trying to acquire the consul lock")
				})

				AfterEach(func() {
					logger.Info("shutting-down-emitter-2")
					session2.Signal(os.Interrupt)
					Eventually(session2.Exited, 5*time.Second).Should(BeClosed())

				})

				Context("and the first emitter goes away", func() {
					BeforeEach(func() {
						logger.Info("forcing-emitter-1-to-shutting-down")
						session1.Signal(os.Interrupt)
					})

					Describe("the second emitter", func() {
						It("becomes active", func() {
							Eventually(session2.Out, 5*time.Second).Should(gbytes.Say("tcp-emitter.started"))

							By("the second emitter could receive events")

							checkEmitterWorks(session2)
							checkTcpRouteMapping(expectedTcpRouteMapping)
						})
					})
				})
			})
		})
	})
})
