package routing_table_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/bbs/fake_bbs"
	"github.com/cloudfoundry-incubator/bbs/models"
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
		fakeBbsClient       *fake_bbs.FakeClient
	)

	BeforeEach(func() {
		fakeRoutingTable = new(fakes.FakeRoutingTable)
		fakeEmitter = new(fakes.FakeEmitter)
		fakeBbsClient = new(fake_bbs.FakeClient)
		routingTableHandler = routing_table.NewRoutingTableHandler(logger, fakeRoutingTable, fakeEmitter, fakeBbsClient)
	})

	Describe("DesiredLRP Event", func() {
		var (
			desiredLRP    *models.DesiredLRP
			routingEvents routing_table.RoutingEvents
		)

		BeforeEach(func() {
			externalPort := uint32(61000)
			containerPort := uint32(5222)
			tcpRoutes := tcp_routes.TCPRoutes{
				tcp_routes.TCPRoute{
					ExternalPort:  externalPort,
					ContainerPort: containerPort,
				},
			}
			desiredLRP = &models.DesiredLRP{
				ProcessGuid: "process-guid-1",
				Ports:       []uint32{containerPort},
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
				routingTableHandler.HandleEvent(models.NewDesiredLRPCreatedEvent(desiredLRP))
			})

			It("invokes AddRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.AddRoutesCallCount()).Should(Equal(1))
				lrp := fakeRoutingTable.AddRoutesArgsForCall(0)
				Expect(lrp).Should(Equal(desiredLRP))
			})

			Context("when there are routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.AddRoutesReturns(routingEvents)
				})

				It("invokes Emit on Emitter", func() {
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(1))
					events := fakeEmitter.EmitArgsForCall(0)
					Expect(events).Should(Equal(routingEvents))
				})
			})

			Context("when there are no routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.AddRoutesReturns(routing_table.RoutingEvents{})
				})

				It("does not invoke Emit on Emitter", func() {
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
				})
			})
		})

		Describe("HandleDesiredUpdate", func() {
			var after *models.DesiredLRP

			BeforeEach(func() {
				externalPort := uint32(62000)
				containerPort := uint32(5222)
				tcpRoutes := tcp_routes.TCPRoutes{
					tcp_routes.TCPRoute{
						ExternalPort:  externalPort,
						ContainerPort: containerPort,
					},
				}
				after = &models.DesiredLRP{
					ProcessGuid: "process-guid-1",
					Ports:       []uint32{containerPort},
					LogGuid:     "log-guid",
					Routes:      tcpRoutes.RoutingInfo(),
				}
			})

			JustBeforeEach(func() {
				routingTableHandler.HandleEvent(models.NewDesiredLRPChangedEvent(desiredLRP, after))
			})

			It("invokes UpdateRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.UpdateRoutesCallCount()).Should(Equal(1))
				beforeLrp, afterLrp := fakeRoutingTable.UpdateRoutesArgsForCall(0)
				Expect(beforeLrp).Should(Equal(desiredLRP))
				Expect(afterLrp).Should(Equal(after))
			})

			Context("when there are routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.UpdateRoutesReturns(routingEvents)
				})

				It("invokes Emit on Emitter", func() {
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(1))
					events := fakeEmitter.EmitArgsForCall(0)
					Expect(events).Should(Equal(routingEvents))
				})
			})

			Context("when there are no routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.UpdateRoutesReturns(routing_table.RoutingEvents{})
				})

				It("does not invoke Emit on Emitter", func() {
					Expect(fakeEmitter.EmitCallCount()).Should(Equal(0))
				})
			})
		})

		Describe("HandleDesiredDelete", func() {
			BeforeEach(func() {
				unregistrationEvent := routing_table.RoutingEvents{
					routing_table.RoutingEvent{
						EventType: routing_table.RouteUnregistrationEvent,
						Key:       routing_table.RoutingKey{},
						Entry:     routing_table.RoutableEndpoints{},
					},
				}
				fakeRoutingTable.RemoveRoutesReturns(unregistrationEvent)
			})
			JustBeforeEach(func() {
				routingTableHandler.HandleEvent(models.NewDesiredLRPRemovedEvent(desiredLRP))
			})

			It("does not invoke AddRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.RemoveRoutesCallCount()).Should(Equal(1))
				Expect(fakeEmitter.EmitCallCount()).Should(Equal(1))
				lrp := fakeRoutingTable.RemoveRoutesArgsForCall(0)
				Expect(lrp).Should(Equal(desiredLRP))
			})
		})
	})

	Describe("ActualLRP Event", func() {
		var (
			actualLRP     *models.ActualLRPGroup
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
				routingTableHandler.HandleEvent(models.NewActualLRPCreatedEvent(actualLRP))
			})

			Context("when state is Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								models.NewPortMapping(611006, 5222),
							),
							State: models.ActualLRPStateRunning,
						},
						Evacuating: nil,
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
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								models.NewPortMapping(611006, 5222),
							),
							State: models.ActualLRPStateClaimed,
						},
						Evacuating: nil,
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
				afterLRP *models.ActualLRPGroup
			)

			JustBeforeEach(func() {
				routingTableHandler.HandleEvent(models.NewActualLRPChangedEvent(actualLRP, afterLRP))
			})

			Context("when after state is Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"",
							),
							State: models.ActualLRPStateClaimed,
						},
						Evacuating: nil,
					}

					afterLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								models.NewPortMapping(611006, 5222),
							),
							State: models.ActualLRPStateRunning,
						},
						Evacuating: nil,
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
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								models.NewPortMapping(611006, 5222),
							),
							State: models.ActualLRPStateRunning,
						},
						Evacuating: nil,
					}

					afterLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"",
							),
							State: models.ActualLRPStateCrashed,
						},
						Evacuating: nil,
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
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", ""),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"",
							),
							State: models.ActualLRPStateUnclaimed,
						},
						Evacuating: nil,
					}

					afterLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"",
							),
							State: models.ActualLRPStateClaimed,
						},
						Evacuating: nil,
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
				routingTableHandler.HandleEvent(models.NewActualLRPRemovedEvent(actualLRP))
			})

			Context("when state is Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								models.NewPortMapping(611006, 5222),
							),
							State: models.ActualLRPStateRunning,
						},
						Evacuating: nil,
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
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"",
							),
							State: models.ActualLRPStateClaimed,
						},
						Evacuating: nil,
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
				desiredLRP  *models.DesiredLRP
			)

			BeforeEach(func() {
				syncChannel = make(chan struct{})
				tmpSyncChannel := syncChannel
				fakeBbsClient.DesiredLRPsStub = func(filter models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
					select {
					case <-tmpSyncChannel:
						logger.Info("Desired LRPs complete")
					}
					return nil, nil
				}
				externalPort := uint32(61000)
				containerPort := uint32(5222)
				tcpRoutes := tcp_routes.TCPRoutes{
					tcp_routes.TCPRoute{
						ExternalPort:  externalPort,
						ContainerPort: containerPort,
					},
				}
				desiredLRP = &models.DesiredLRP{
					ProcessGuid: "process-guid-1",
					Ports:       []uint32{containerPort},
					LogGuid:     "log-guid",
					Routes:      tcpRoutes.RoutingInfo(),
				}
			})

			It("caches the events", func() {
				go invokeSync(doneChannel)
				Eventually(routingTableHandler.Syncing).Should(BeTrue())

				Expect(fakeRoutingTable.AddRoutesCallCount()).Should(Equal(0))
				routingTableHandler.HandleEvent(models.NewDesiredLRPCreatedEvent(desiredLRP))
				Consistently(fakeRoutingTable.AddRoutesCallCount()).Should(Equal(0))
				Eventually(logger).Should(gbytes.Say("test.caching-event"))

				close(syncChannel)
				Eventually(routingTableHandler.Syncing).Should(BeFalse())
				Eventually(fakeRoutingTable.AddRoutesCallCount()).Should(Equal(1))
				Eventually(doneChannel).Should(BeClosed())
				Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(0))
			})
		})

		Context("when bbs server returns error while fetching desired lrps", func() {
			BeforeEach(func() {
				fakeBbsClient.DesiredLRPsReturns(nil, errors.New("kaboom"))
			})

			It("does not update the routing table", func() {
				go invokeSync(doneChannel)
				Eventually(doneChannel).Should(BeClosed())
				Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(0))
				Eventually(logger).Should(gbytes.Say("test.handle-sync.failed-getting-desired-lrps"))
			})

		})

		Context("when bbs server returns error while fetching actual lrps", func() {
			BeforeEach(func() {
				fakeBbsClient.ActualLRPGroupsReturns(nil, errors.New("kaboom"))
			})

			It("does not update the routing table", func() {
				go invokeSync(doneChannel)
				Eventually(doneChannel).Should(BeClosed())
				Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(0))
				Eventually(logger).Should(gbytes.Say("test.handle-sync.failed-getting-actual-lrps"))
			})
		})

		Context("when bbs server calls return successfully", func() {
			Context("when bbs server returns no data", func() {
				It("does not update the routing table", func() {
					go invokeSync(doneChannel)
					Eventually(doneChannel).Should(BeClosed())
					Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(0))
				})
			})

			Context("when bbs server returns desired and actual lrps", func() {

				var (
					desiredLRP      *models.DesiredLRP
					actualLRP       *models.ActualLRPGroup
					modificationTag models.ModificationTag
				)

				BeforeEach(func() {
					modificationTag = models.ModificationTag{Epoch: "abc", Index: 1}
					externalPort := uint32(61000)
					containerPort := uint32(5222)
					tcpRoutes := tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    externalPort,
							ContainerPort:   containerPort,
						},
					}

					desiredLRP = &models.DesiredLRP{
						ProcessGuid:     "process-guid-1",
						Ports:           []uint32{containerPort},
						LogGuid:         "log-guid",
						Routes:          tcpRoutes.RoutingInfo(),
						ModificationTag: &modificationTag,
					}
					actualLRP = &models.ActualLRPGroup{
						Instance: &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("process-guid-1", 0, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
							ActualLRPNetInfo: models.NewActualLRPNetInfo(
								"some-ip",
								models.NewPortMapping(61006, 5222),
							),
							State:           models.ActualLRPStateRunning,
							ModificationTag: modificationTag,
						},
						Evacuating: nil,
					}
					fakeBbsClient.DesiredLRPsReturns([]*models.DesiredLRP{desiredLRP}, nil)
					fakeBbsClient.ActualLRPGroupsReturns([]*models.ActualLRPGroup{actualLRP}, nil)

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
						afterActualLRP       *models.ActualLRPGroup
						afterModificationTag models.ModificationTag
					)

					BeforeEach(func() {
						afterModificationTag = models.ModificationTag{Epoch: "abc", Index: 2}
						afterActualLRP = &models.ActualLRPGroup{
							Instance: &models.ActualLRP{
								ActualLRPKey:         models.NewActualLRPKey("process-guid-1", 0, "domain"),
								ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id-1"),
								ActualLRPNetInfo: models.NewActualLRPNetInfo(
									"some-ip-1",
									models.NewPortMapping(61007, 5222),
								),
								State:           models.ActualLRPStateRunning,
								ModificationTag: afterModificationTag,
							},
							Evacuating: nil,
						}
						syncChannel = make(chan struct{})
						tmpSyncChannel := syncChannel
						fakeBbsClient.DesiredLRPsStub = func(filter models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
							select {
							case <-tmpSyncChannel:
								logger.Info("Desired LRPs complete")
							}
							return []*models.DesiredLRP{desiredLRP}, nil
						}
					})

					It("caches events and applies it to new routing table", func() {
						go invokeSync(doneChannel)
						Eventually(routingTableHandler.Syncing).Should(BeTrue())

						Expect(fakeRoutingTable.AddRoutesCallCount()).Should(Equal(0))
						routingTableHandler.HandleEvent(models.NewActualLRPChangedEvent(actualLRP, afterActualLRP))
						Consistently(fakeRoutingTable.AddRoutesCallCount()).Should(Equal(0))
						Eventually(logger).Should(gbytes.Say("test.caching-event"))

						close(syncChannel)
						Eventually(routingTableHandler.Syncing).Should(BeFalse())
						Expect(fakeRoutingTable.AddRoutesCallCount()).Should(Equal(0))
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
								"instance-guid", false, "some-ip-1", 61007, 5222, &afterModificationTag),
						}

						Expect(routingEvent.Key).Should(Equal(key))
						Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
						externalInfo := []routing_table.ExternalEndpointInfo{
							routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
						}
						expectedEntry := routing_table.NewRoutableEndpoints(
							externalInfo, endpoints, "log-guid", &modificationTag)
						Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					})

				})
			})
		})

	})
})
