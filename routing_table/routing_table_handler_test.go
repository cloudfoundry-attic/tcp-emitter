package routing_table_test

import (
	"errors"
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/receptor/fake_receptor"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table/fakes"
	"github.com/cloudfoundry-incubator/tcp-emitter/tcp_routes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("RoutingTableHandler", func() {
	var (
		fakeRoutingTable    *fakes.FakeRoutingTable
		fakeEmitter         *fakes.FakeEmitter
		routingTableHandler routing_table.RoutingTableHandler
		fakeReceptorClient  *fake_receptor.FakeClient
	)

	BeforeEach(func() {
		fakeRoutingTable = new(fakes.FakeRoutingTable)
		fakeEmitter = new(fakes.FakeEmitter)
		fakeReceptorClient = new(fake_receptor.FakeClient)
		routingTableHandler = routing_table.NewRoutingTableHandler(logger, fakeRoutingTable, fakeEmitter, fakeReceptorClient)
	})

	Describe("DesiredLRP Event", func() {
		var (
			desiredLRP    receptor.DesiredLRPResponse
			routingEvents routing_table.RoutingEvents
		)

		BeforeEach(func() {
			externalPort := uint16(61000)
			containerPort := uint16(5222)
			tcpRoutes := tcp_routes.TCPRoutes{
				tcp_routes.TCPRoute{
					ExternalPort:  externalPort,
					ContainerPort: containerPort,
				},
			}
			desiredLRP = receptor.DesiredLRPResponse{
				ProcessGuid: "process-guid-1",
				Ports:       []uint16{containerPort},
				LogGuid:     "log-guid",
				Routes:      tcpRoutes.RoutingInfo(),
			}
			routingEvents = routing_table.RoutingEvents{
				routing_table.RoutingEvent{
					EventType: routing_table.RouteRegistrationEvent,
					Key:       routing_table.RoutingKey{},
					Entry:     routing_table.RoutableEndpoints{},
				},
			}
		})

		Describe("HandleDesiredCreate", func() {
			JustBeforeEach(func() {
				routingTableHandler.HandleEvent(receptor.NewDesiredLRPCreatedEvent(desiredLRP))
			})

			It("invokes SetRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(1))
				lrp := fakeRoutingTable.SetRoutesArgsForCall(0)
				Expect(lrp).Should(Equal(desiredLRP))
			})

			Context("when there are routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.SetRoutesReturns(routingEvents)
				})

				It("invokes Emit on Emitter", func() {
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(1))
					events := fakeEmitter.EmitArgsForCall(0)
					Expect(events).Should(Equal(routingEvents))
				})
			})

			Context("when there are no routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.SetRoutesReturns(routing_table.RoutingEvents{})
				})

				It("does not invoke Emit on Emitter", func() {
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
				})
			})
		})

		Describe("HandleDesiredUpdate", func() {
			var after receptor.DesiredLRPResponse

			BeforeEach(func() {
				externalPort := uint16(62000)
				containerPort := uint16(5222)
				tcpRoutes := tcp_routes.TCPRoutes{
					tcp_routes.TCPRoute{
						ExternalPort:  externalPort,
						ContainerPort: containerPort,
					},
				}
				after = receptor.DesiredLRPResponse{
					ProcessGuid: "process-guid-1",
					Ports:       []uint16{containerPort},
					LogGuid:     "log-guid",
					Routes:      tcpRoutes.RoutingInfo(),
				}
			})

			JustBeforeEach(func() {
				routingTableHandler.HandleEvent(receptor.NewDesiredLRPChangedEvent(desiredLRP, after))
			})

			It("invokes SetRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(1))
				lrp := fakeRoutingTable.SetRoutesArgsForCall(0)
				Expect(lrp).Should(Equal(after))
			})

			Context("when there are routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.SetRoutesReturns(routingEvents)
				})

				It("invokes Emit on Emitter", func() {
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(1))
					events := fakeEmitter.EmitArgsForCall(0)
					Expect(events).Should(Equal(routingEvents))
				})
			})

			Context("when there are no routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.SetRoutesReturns(routing_table.RoutingEvents{})
				})

				It("does not invoke Emit on Emitter", func() {
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
				})
			})
		})

		Describe("HandleDesiredDelete", func() {
			JustBeforeEach(func() {
				routingTableHandler.HandleEvent(receptor.NewDesiredLRPRemovedEvent(desiredLRP))
			})

			It("does not invoke SetRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(0))
				Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
			})
		})
	})

	Describe("ActualLRP Event", func() {
		var (
			actualLRP     receptor.ActualLRPResponse
			routingEvents routing_table.RoutingEvents
		)

		BeforeEach(func() {

			routingEvents = routing_table.RoutingEvents{
				routing_table.RoutingEvent{
					EventType: routing_table.RouteRegistrationEvent,
					Key:       routing_table.RoutingKey{},
					Entry:     routing_table.RoutableEndpoints{},
				},
			}
		})

		Describe("HandleActualCreate", func() {
			JustBeforeEach(func() {
				routingTableHandler.HandleEvent(receptor.NewActualLRPCreatedEvent(actualLRP))
			})

			Context("when state is Running", func() {
				BeforeEach(func() {
					actualLRP = receptor.ActualLRPResponse{
						ProcessGuid:  "process-guid",
						InstanceGuid: "instance-guid",
						Address:      "some-ip",
						Evacuating:   false,
						State:        receptor.ActualLRPStateRunning,
						Ports:        []receptor.PortMapping{{ContainerPort: 5222, HostPort: 61106}},
					}
				})

				It("invokes AddEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(1))
					lrp := fakeRoutingTable.AddEndpointArgsForCall(0)
					Expect(lrp).Should(Equal(actualLRP))
				})

				Context("when there are routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.AddEndpointReturns(routingEvents)
					})

					It("invokes Emit on Emitter", func() {
						Expect(fakeEmitter.EmitCallCount()).Should(Equal(1))
						events := fakeEmitter.EmitArgsForCall(0)
						Expect(events).Should(Equal(routingEvents))
					})
				})

				Context("when there are no routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.AddEndpointReturns(routing_table.RoutingEvents{})
					})

					It("does not invoke Emit on Emitter", func() {
						Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
					})
				})
			})

			Context("when state is not in Running", func() {
				BeforeEach(func() {
					actualLRP = receptor.ActualLRPResponse{
						ProcessGuid:  "process-guid",
						InstanceGuid: "instance-guid",
						Address:      "some-ip",
						Evacuating:   false,
						State:        receptor.ActualLRPStateClaimed,
						Ports:        []receptor.PortMapping{{ContainerPort: 5222, HostPort: 61106}},
					}
				})

				It("does not invoke AddEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(0))
				})

				It("does not invoke Emit on Emitter", func() {
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
				})
			})
		})

		Describe("HandleActualUpdate", func() {
			var (
				afterLRP receptor.ActualLRPResponse
			)

			JustBeforeEach(func() {
				routingTableHandler.HandleEvent(receptor.NewActualLRPChangedEvent(actualLRP, afterLRP))
			})

			Context("when after state is Running", func() {
				BeforeEach(func() {
					actualLRP = receptor.ActualLRPResponse{
						ProcessGuid:  "process-guid",
						InstanceGuid: "instance-guid",
						Address:      "",
						Evacuating:   false,
						State:        receptor.ActualLRPStateClaimed,
						Ports:        []receptor.PortMapping{},
					}

					afterLRP = receptor.ActualLRPResponse{
						ProcessGuid:  "process-guid",
						InstanceGuid: "instance-guid",
						Address:      "some-ip",
						Evacuating:   false,
						State:        receptor.ActualLRPStateRunning,
						Ports:        []receptor.PortMapping{{ContainerPort: 5222, HostPort: 61106}},
					}
				})

				It("invokes AddEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(1))
					lrp := fakeRoutingTable.AddEndpointArgsForCall(0)
					Expect(lrp).Should(Equal(afterLRP))
				})

				Context("when there are routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.AddEndpointReturns(routingEvents)
					})

					It("invokes Emit on Emitter", func() {
						Expect(fakeEmitter.EmitCallCount()).Should(Equal(1))
						events := fakeEmitter.EmitArgsForCall(0)
						Expect(events).Should(Equal(routingEvents))
					})
				})

				Context("when there are no routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.AddEndpointReturns(routing_table.RoutingEvents{})
					})

					It("does not invoke Emit on Emitter", func() {
						Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
					})
				})
			})

			Context("when after state is not Running and before state is Running", func() {
				BeforeEach(func() {
					actualLRP = receptor.ActualLRPResponse{
						ProcessGuid:  "process-guid",
						InstanceGuid: "instance-guid",
						Address:      "some-ip",
						Evacuating:   false,
						State:        receptor.ActualLRPStateRunning,
						Ports:        []receptor.PortMapping{{ContainerPort: 5222, HostPort: 61106}},
					}

					afterLRP = receptor.ActualLRPResponse{
						ProcessGuid:  "process-guid",
						InstanceGuid: "instance-guid",
						Address:      "",
						Evacuating:   false,
						State:        receptor.ActualLRPStateCrashed,
						Ports:        []receptor.PortMapping{},
					}
				})

				It("invokes RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(1))
					lrp := fakeRoutingTable.RemoveEndpointArgsForCall(0)
					Expect(lrp).Should(Equal(actualLRP))
				})

				Context("when there are routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.RemoveEndpointReturns(routingEvents)
					})

					It("invokes Emit on Emitter", func() {
						Expect(fakeEmitter.EmitCallCount()).Should(Equal(1))
						events := fakeEmitter.EmitArgsForCall(0)
						Expect(events).Should(Equal(routingEvents))
					})
				})

				Context("when there are no routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.RemoveEndpointReturns(routing_table.RoutingEvents{})
					})

					It("does not invoke Emit on Emitter", func() {
						Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
					})
				})
			})

			Context("when both after and before state is not Running", func() {
				BeforeEach(func() {
					actualLRP = receptor.ActualLRPResponse{
						ProcessGuid:  "process-guid",
						InstanceGuid: "instance-guid",
						Address:      "",
						Evacuating:   false,
						State:        receptor.ActualLRPStateUnclaimed,
						Ports:        []receptor.PortMapping{},
					}

					afterLRP = receptor.ActualLRPResponse{
						ProcessGuid:  "process-guid",
						InstanceGuid: "instance-guid",
						Address:      "",
						Evacuating:   false,
						State:        receptor.ActualLRPStateClaimed,
						Ports:        []receptor.PortMapping{},
					}
				})

				It("does not invoke AddEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(0))
				})

				It("does not invoke RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(0))
				})

				It("does not invoke Emit on Emitter", func() {
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
				})
			})
		})

		Describe("HandleActualDelete", func() {
			JustBeforeEach(func() {
				routingTableHandler.HandleEvent(receptor.NewActualLRPRemovedEvent(actualLRP))
			})

			Context("when state is Running", func() {
				BeforeEach(func() {
					actualLRP = receptor.ActualLRPResponse{
						ProcessGuid:  "process-guid",
						InstanceGuid: "instance-guid",
						Address:      "some-ip",
						Evacuating:   false,
						State:        receptor.ActualLRPStateRunning,
						Ports:        []receptor.PortMapping{{ContainerPort: 5222, HostPort: 61106}},
					}
				})

				It("invokes RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(1))
					lrp := fakeRoutingTable.RemoveEndpointArgsForCall(0)
					Expect(lrp).Should(Equal(actualLRP))
				})

				Context("when there are routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.RemoveEndpointReturns(routingEvents)
					})

					It("invokes Emit on Emitter", func() {
						Expect(fakeEmitter.EmitCallCount()).Should(Equal(1))
						events := fakeEmitter.EmitArgsForCall(0)
						Expect(events).Should(Equal(routingEvents))
					})
				})

				Context("when there are no routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.RemoveEndpointReturns(routing_table.RoutingEvents{})
					})

					It("does not invoke Emit on Emitter", func() {
						Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
					})
				})
			})

			Context("when state is not in Running", func() {
				BeforeEach(func() {
					actualLRP = receptor.ActualLRPResponse{
						ProcessGuid:  "process-guid",
						InstanceGuid: "instance-guid",
						Address:      "some-ip",
						Evacuating:   false,
						State:        receptor.ActualLRPStateClaimed,
						Ports:        []receptor.PortMapping{{ContainerPort: 5222, HostPort: 61106}},
					}
				})

				It("does not invoke RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(0))
				})

				It("does not invoke Emit on Emitter", func() {
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
				})
			})
		})
	})

	Describe("Sync", func() {

		var (
			doneChannel chan struct{}
		)

		invokeSync := func(doneChannel chan struct{}) {
			defer GinkgoRecover()
			routingTableHandler.Sync()
			close(doneChannel)
		}

		BeforeEach(func() {
			doneChannel = make(chan struct{})
		})

		Context("when events are received", func() {
			var (
				syncChannel chan struct{}
				desiredLRP  receptor.DesiredLRPResponse
			)

			BeforeEach(func() {
				syncChannel = make(chan struct{})
				tmpSyncChannel := syncChannel
				fakeReceptorClient.DesiredLRPsStub = func() ([]receptor.DesiredLRPResponse, error) {
					select {
					case <-tmpSyncChannel:
						logger.Info("Desired LRPs complete")
					}
					return nil, nil
				}
				externalPort := uint16(61000)
				containerPort := uint16(5222)
				tcpRoutes := tcp_routes.TCPRoutes{
					tcp_routes.TCPRoute{
						ExternalPort:  externalPort,
						ContainerPort: containerPort,
					},
				}
				desiredLRP = receptor.DesiredLRPResponse{
					ProcessGuid: "process-guid-1",
					Ports:       []uint16{containerPort},
					LogGuid:     "log-guid",
					Routes:      tcpRoutes.RoutingInfo(),
				}
			})

			It("caches the events", func() {
				go invokeSync(doneChannel)
				Eventually(routingTableHandler.Syncing).Should(BeTrue())

				Expect(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(0))
				routingTableHandler.HandleEvent(receptor.NewDesiredLRPCreatedEvent(desiredLRP))
				Consistently(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(0))
				Eventually(logger).Should(gbytes.Say("test.caching-event"))

				close(syncChannel)
				Eventually(routingTableHandler.Syncing).Should(BeFalse())
				Eventually(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(1))
				Eventually(doneChannel).Should(BeClosed())
				Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(0))
			})
		})

		Context("when receptor returns error while fetching desired lrps", func() {
			BeforeEach(func() {
				fakeReceptorClient.DesiredLRPsReturns(nil, errors.New("kaboom"))
			})

			It("does not update the routing table", func() {
				go invokeSync(doneChannel)
				Eventually(doneChannel).Should(BeClosed())
				Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(0))
				Eventually(logger).Should(gbytes.Say("test.handle-sync.failed-getting-desired-lrps"))
			})

		})

		Context("when receptor returns error while fetching actual lrps", func() {
			BeforeEach(func() {
				fakeReceptorClient.ActualLRPsReturns(nil, errors.New("kaboom"))
			})

			It("does not update the routing table", func() {
				go invokeSync(doneChannel)
				Eventually(doneChannel).Should(BeClosed())
				Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(0))
				Eventually(logger).Should(gbytes.Say("test.handle-sync.failed-getting-actual-lrps"))
			})
		})

		Context("when receptor calls return successfully", func() {
			Context("when receptor returns no data", func() {
				It("does not update the routing table", func() {
					go invokeSync(doneChannel)
					Eventually(doneChannel).Should(BeClosed())
					Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(0))
				})
			})

			Context("when receptor returns desired and actual lrps", func() {

				var (
					desiredLRP      receptor.DesiredLRPResponse
					actualLRP       receptor.ActualLRPResponse
					modificationTag receptor.ModificationTag
				)

				BeforeEach(func() {
					modificationTag = receptor.ModificationTag{Epoch: "abc", Index: 1}
					externalPort := uint16(61000)
					containerPort := uint16(5222)
					tcpRoutes := tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							ExternalPort:  externalPort,
							ContainerPort: containerPort,
						},
					}

					desiredLRP = receptor.DesiredLRPResponse{
						ProcessGuid:     "process-guid-1",
						Ports:           []uint16{containerPort},
						LogGuid:         "log-guid",
						Routes:          tcpRoutes.RoutingInfo(),
						ModificationTag: modificationTag,
					}
					actualLRP = receptor.ActualLRPResponse{
						ProcessGuid:     "process-guid-1",
						InstanceGuid:    "instance-guid",
						Address:         "some-ip",
						Evacuating:      false,
						State:           receptor.ActualLRPStateRunning,
						Ports:           []receptor.PortMapping{{ContainerPort: 5222, HostPort: 61106}},
						ModificationTag: modificationTag,
					}
					fakeReceptorClient.DesiredLRPsReturns([]receptor.DesiredLRPResponse{desiredLRP}, nil)
					fakeReceptorClient.ActualLRPsReturns([]receptor.ActualLRPResponse{actualLRP}, nil)

					fakeRoutingTable.SwapStub = func(t routing_table.RoutingTable) routing_table.RoutingEvents {
						routingEvents := routing_table.RoutingEvents{
							routing_table.RoutingEvent{
								EventType: routing_table.RouteRegistrationEvent,
								Key:       routing_table.RoutingKey{},
								Entry:     routing_table.RoutableEndpoints{},
							},
						}
						return routingEvents
					}
				})

				It("updates the routing table", func() {
					go invokeSync(doneChannel)
					Eventually(doneChannel).Should(BeClosed())
					Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(1))
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(1))
				})

				Context("when events are received", func() {

					var (
						syncChannel          chan struct{}
						afterActualLRP       receptor.ActualLRPResponse
						afterModificationTag receptor.ModificationTag
					)

					BeforeEach(func() {
						afterModificationTag = receptor.ModificationTag{Epoch: "abc", Index: 2}
						afterActualLRP = receptor.ActualLRPResponse{
							ProcessGuid:     "process-guid-1",
							InstanceGuid:    "instance-guid",
							Address:         "some-ip-1",
							Evacuating:      false,
							State:           receptor.ActualLRPStateRunning,
							Ports:           []receptor.PortMapping{{ContainerPort: 5222, HostPort: 61107}},
							ModificationTag: afterModificationTag,
						}
						syncChannel = make(chan struct{})
						tmpSyncChannel := syncChannel
						fakeReceptorClient.DesiredLRPsStub = func() ([]receptor.DesiredLRPResponse, error) {
							select {
							case <-tmpSyncChannel:
								logger.Info("Desired LRPs complete")
							}
							return []receptor.DesiredLRPResponse{desiredLRP}, nil
						}
					})

					It("caches events and applies it to new routing table", func() {
						go invokeSync(doneChannel)
						Eventually(routingTableHandler.Syncing).Should(BeTrue())

						Expect(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(0))
						routingTableHandler.HandleEvent(receptor.NewActualLRPChangedEvent(actualLRP, afterActualLRP))
						Consistently(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(0))
						Eventually(logger).Should(gbytes.Say("test.caching-event"))

						close(syncChannel)
						Eventually(routingTableHandler.Syncing).Should(BeFalse())
						Expect(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(0))
						Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(1))
						Expect(fakeEmitter.EmitCallCount()).Should(Equal(1))

						tempRoutingTable := fakeRoutingTable.SwapArgsForCall(0)
						Expect(tempRoutingTable.RouteCount()).To(Equal(1))
						routingEvents := tempRoutingTable.GetRoutingEvents()
						Expect(routingEvents).To(HaveLen(1))
						routingEvent := routingEvents[0]

						key := routing_table.RoutingKey{
							ProcessGuid:   "process-guid-1",
							ContainerPort: 5222,
						}
						endpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
							routing_table.NewEndpointKey("instance-guid", false): routing_table.NewEndpoint(
								"instance-guid", false, "some-ip-1", 61107, 5222, afterModificationTag),
						}

						Expect(routingEvent.Key).Should(Equal(key))
						Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
						externalInfo := []routing_table.ExternalEndpointInfo{
							routing_table.NewExternalEndpointInfo(61000),
						}
						expectedEntry := routing_table.NewRoutableEndpoints(
							externalInfo, endpoints, "log-guid", modificationTag)
						Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					})

				})
			})
		})

	})
})
