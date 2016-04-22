package main_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/bbs/models"

	"github.com/cloudfoundry-incubator/routing-info/tcp_routes"
	"github.com/gogo/protobuf/proto"
	"github.com/tedsuo/ifrit"
	"github.com/vito/go-sse/sse"

	"github.com/cloudfoundry-incubator/routing-api"
	routingtestrunner "github.com/cloudfoundry-incubator/routing-api/cmd/routing-api/testrunner"
	apimodels "github.com/cloudfoundry-incubator/routing-api/models"
	"github.com/cloudfoundry-incubator/tcp-emitter/cmd/tcp-emitter/testrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("TCP Emitter", func() {

	Describe("Main", func() {

		var (
			expectedTcpRouteMapping    apimodels.TcpRouteMapping
			notExpectedTcpRouteMapping apimodels.TcpRouteMapping
		)

		getDesiredLRP := func(processGuid, logGuid, routerGroupGuid string, externalPort, containerPort, modificationIndex uint32) models.DesiredLRP {
			desiredLRP := models.DesiredLRP{}
			desiredLRP.ProcessGuid = processGuid
			desiredLRP.Ports = []uint32{containerPort}
			desiredLRP.LogGuid = logGuid
			tcpRoutes := tcp_routes.TCPRoutes{
				tcp_routes.TCPRoute{
					RouterGroupGuid: routerGroupGuid,
					ExternalPort:    externalPort,
					ContainerPort:   containerPort,
				},
			}
			desiredLRP.Routes = tcpRoutes.RoutingInfo()
			desiredLRP.ModificationTag = &models.ModificationTag{Epoch: "abc", Index: modificationIndex}
			return desiredLRP
		}

		getActualLRP := func(processGuid, instanceGuid, ipAddress string, containerPort uint32) models.ActualLRPGroup {
			return models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						ipAddress,
						models.NewPortMapping(62003, containerPort),
					),
					State: models.ActualLRPStateRunning,
				},
				Evacuating: nil,
			}
		}

		setupBbsServer := func(server *ghttp.Server, includeSecondLRP, emitEvents bool, exitChannel chan struct{}, routerGroupGuid string) {
			server.RouteToHandler("POST", "/v1/actual_lrp_groups/list",
				func(w http.ResponseWriter, req *http.Request) {
					actualLRP1 := getActualLRP("some-guid", "instance-guid", "some-ip", 5222)
					actualLRPs := []*models.ActualLRPGroup{
						&actualLRP1,
					}
					if includeSecondLRP {
						actualLRP2 := getActualLRP("some-guid-1", "instance-guid-1", "some-ip-1", 1883)
						actualLRPs = append(actualLRPs, &actualLRP2)
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
					desiredLRP1 := getDesiredLRP("some-guid", "log-guid", routerGroupGuid, 5222, 5222, 1)
					desiredLRPs := []*models.DesiredLRP{
						&desiredLRP1,
					}
					if includeSecondLRP {
						desiredLRP2 := getDesiredLRP("some-guid-1", "log-guid-1", routerGroupGuid, 1883, 1883, 1)
						desiredLRPs = append(desiredLRPs, &desiredLRP2)
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

			deletedDesiredLRP := getDesiredLRP("some-guid-1", "log-guid-1", routerGroupGuid, 1883, 1883, 2)
			desiredLRPEvent := models.NewDesiredLRPRemovedEvent(&deletedDesiredLRP)
			eventData, err := proto.Marshal(desiredLRPEvent)
			b64EventData := base64.StdEncoding.EncodeToString(eventData)

			Expect(err).ToNot(HaveOccurred())
			sseEvent := sse.Event{
				ID:   "1",
				Name: models.EventTypeDesiredLRPRemoved,
				Data: []byte(b64EventData),
			}
			server.RouteToHandler("GET", "/v1/events",
				func(w http.ResponseWriter, req *http.Request) {
					flusher := w.(http.Flusher)
					headers := w.Header()
					headers["Content-Type"] = []string{"text/event-stream; charset=utf-8"}
					w.WriteHeader(http.StatusOK)
					flusher.Flush()
					for {
						select {
						case <-exitChannel:
							return
						default:
							sseEvent.Write(w)
							flusher.Flush()
							time.Sleep(1 * time.Second)
						}
					}
				})
		}

		getRouterGroupGuid := func(port uint16) string {
			client := routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", port))
			routerGroups, err := client.RouterGroups()
			Expect(err).NotTo(HaveOccurred())
			Expect(routerGroups).ToNot(HaveLen(0))
			return routerGroups[0].Guid
		}

		setupRoutingApiServer := func(path string, args routingtestrunner.Args) (ifrit.Process, string) {
			routingApiServer := routingtestrunner.New(path, args)
			process := ifrit.Invoke(routingApiServer)
			routerGroupGuid := getRouterGroupGuid(args.Port)
			expectedTcpRouteMapping.TcpRoute.RouterGroupGuid = routerGroupGuid
			notExpectedTcpRouteMapping.TcpRoute.RouterGroupGuid = routerGroupGuid
			return process, routerGroupGuid
		}

		setupTcpEmitter := func(path string, args testrunner.Args, expectStarted bool) *gexec.Session {
			allOutput := gbytes.NewBuffer()
			runner := testrunner.New(path, args)
			session, err := gexec.Start(runner.Command, allOutput, allOutput)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("setting-up-bbs-client.*bbsURL"))

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
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("syncer.syncing"))
			Consistently(session.Out, 5*time.Second).ShouldNot(gbytes.Say("unable-to-upsert"))
			Eventually(session.Out, 5*time.Second).Should(gbytes.Say("successfully-emitted-events"))
		}

		checkTcpRouteMapping := func(tcpRouteMapping apimodels.TcpRouteMapping, present bool) {
			if present {
				Eventually(func() bool {
					mappings, _ := routingApiClient.TcpRouteMappings()
					return contains(mappings, tcpRouteMapping)
				}, 5*time.Second).Should(BeTrue())
			} else {
				Eventually(func() []apimodels.TcpRouteMapping {
					tcpRouteMappings, _ := routingApiClient.TcpRouteMappings()
					return tcpRouteMappings
				}, 5*time.Second).ShouldNot(ContainElement(tcpRouteMapping))
			}
		}

		BeforeEach(func() {
			expectedTcpRouteMapping = apimodels.TcpRouteMapping{
				TcpRoute: apimodels.TcpRoute{
					ExternalPort: 5222,
				},
				HostPort: 62003,
				HostIP:   "some-ip",
			}
			notExpectedTcpRouteMapping = apimodels.TcpRouteMapping{
				TcpRoute: apimodels.TcpRoute{
					ExternalPort: 1883,
				},
				HostPort: 62003,
				HostIP:   "some-ip-1",
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

		Context("when there is an error fetching token from uaa", func() {
			var (
				session *gexec.Session
			)

			BeforeEach(func() {
				tcpEmitterArgs := testrunner.Args{
					BBSAddress:     bbsServer.URL(),
					BBSClientCert:  "",
					BBSCACert:      "",
					BBSClientKey:   "",
					ConfigFilePath: createEmitterConfig("33333"),
					SyncInterval:   1 * time.Second,
					ConsulCluster:  consulRunner.ConsulCluster(),
				}
				session = setupTcpEmitter(tcpEmitterBinPath, tcpEmitterArgs, false)
			})

			It("exits with error", func() {
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("error-fetching-oauth-token"))
				Eventually(session.Exited, 5*time.Second).Should(BeClosed())
			})
		})

		Context("when both bbs and routing api server are up and running", func() {
			var (
				routingApiProcess ifrit.Process
				session           *gexec.Session
				exitChannel       chan struct{}
				routerGroupGuid   string
			)
			BeforeEach(func() {
				exitChannel = make(chan struct{})
				routingApiProcess, routerGroupGuid = setupRoutingApiServer(routingAPIBinPath, routingAPIArgs)
				setupBbsServer(bbsServer, true, true, exitChannel, routerGroupGuid)
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
				close(exitChannel)
			})

			It("starts an SSE connection to the bbs and emits events to routing api", func() {
				checkEmitterWorks(session)
				Eventually(session.Out, 2*time.Second).Should(gbytes.Say("successfully-emitted-registration-events"))
				checkTcpRouteMapping(expectedTcpRouteMapping, true)

				Eventually(session.Out, 2*time.Second).Should(gbytes.Say("successfully-emitted-unregistration-events"))
				checkTcpRouteMapping(notExpectedTcpRouteMapping, false)
			})
		})

		Context("when routing api server is down but bbs is running", func() {
			var (
				routingApiProcess ifrit.Process
				session           *gexec.Session
				exitChannel       chan struct{}
			)

			BeforeEach(func() {
				exitChannel = make(chan struct{})
				setupBbsServer(bbsServer, false, true, exitChannel, "some-guid")
				session = setupTcpEmitter(tcpEmitterBinPath, tcpEmitterArgs, true)
				logger.Info("started-tcp-emitter")
			})

			AfterEach(func() {
				logger.Info("shutting-down")
				session.Signal(os.Interrupt)
				Eventually(session.Exited, 5*time.Second).Should(BeClosed())
				routingApiProcess.Signal(os.Interrupt)
				Eventually(routingApiProcess.Wait(), 5*time.Second).Should(Receive())
				close(exitChannel)
			})

			It("starts an SSE connection to the bbs and continues to try to emit to routing api", func() {
				Eventually(eventsEndpointRequests, 5*time.Second).Should(BeNumerically(">=", 1))

				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("subscribed-to-bbs-event"))
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("syncer.syncing"))
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("unable-to-upsert.*connection refused"))
				Consistently(session.Out, 5*time.Second).ShouldNot(gbytes.Say("successfully-emitted-event"))
				Consistently(session.Exited).ShouldNot(BeClosed())

				By("starting routing api server")
				routingApiProcess, _ = setupRoutingApiServer(routingAPIBinPath, routingAPIArgs)
				logger.Info("started-routing-api-server")
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("unable-to-upsert.*some-guid not found"))
			})

		})

		Context("when bbs server is down but routing api is running", func() {
			var (
				routingApiProcess ifrit.Process
				session           *gexec.Session
			)

			BeforeEach(func() {
				routingApiProcess, _ = setupRoutingApiServer(routingAPIBinPath, routingAPIArgs)
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
				exitChannel       chan struct{}
				routerGroupGuid   string
			)
			BeforeEach(func() {
				exitChannel = make(chan struct{})
				routingApiProcess, routerGroupGuid = setupRoutingApiServer(routingAPIBinPath, routingAPIArgs)
				setupBbsServer(bbsServer, false, true, exitChannel, routerGroupGuid)
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
				close(exitChannel)
			})

			It("and the first emitter starts an SSE connection to the bbs and emits events to routing api", func() {
				checkEmitterWorks(session1)
				checkTcpRouteMapping(expectedTcpRouteMapping, true)
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
							checkTcpRouteMapping(expectedTcpRouteMapping, true)
						})
					})
				})
			})
		})

		Context("when routing api auth is disabled", func() {
			var (
				routingApiProcess ifrit.Process
				session           *gexec.Session
				exitChannel       chan struct{}
				routerGroupGuid   string
			)
			BeforeEach(func() {
				exitChannel = make(chan struct{})
				routingApiProcess, routerGroupGuid = setupRoutingApiServer(routingAPIBinPath, routingAPIArgs)
				setupBbsServer(bbsServer, true, false, exitChannel, routerGroupGuid)
				logger.Info("started-routing-api-server")
				unAuthTcpEmitterArgs := testrunner.Args{
					BBSAddress:     bbsServer.URL(),
					BBSClientCert:  createClientCert(),
					BBSCACert:      createCACert(),
					BBSClientKey:   createClientKey(),
					ConfigFilePath: createEmitterConfigAuthDisabled(),
					SyncInterval:   1 * time.Second,
					ConsulCluster:  consulRunner.ConsulCluster(),
				}

				allOutput := gbytes.NewBuffer()
				runner := testrunner.New(tcpEmitterBinPath, unAuthTcpEmitterArgs)
				var err error
				session, err = gexec.Start(runner.Command, allOutput, allOutput)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				logger.Info("shutting-down")
				session.Signal(os.Interrupt)
				Eventually(session.Exited, 5*time.Second).Should(BeClosed())
				routingApiProcess.Signal(os.Interrupt)
				Eventually(routingApiProcess.Wait(), 5*time.Second).Should(Receive())
				close(exitChannel)
			})

			It("does not call oauth server to get the auth token", func() {
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("creating-noop-uaa-client"))
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("tcp-emitter.started"))
				Eventually(session.Out, 2*time.Second).Should(gbytes.Say("successfully-emitted-registration-events"))
				checkTcpRouteMapping(expectedTcpRouteMapping, true)
			})
		})
	})
})

func contains(ms []apimodels.TcpRouteMapping, tcpRouteMapping apimodels.TcpRouteMapping) bool {
	for _, m := range ms {
		if m.Matches(tcpRouteMapping) {
			return true
		}
	}
	return false
}
