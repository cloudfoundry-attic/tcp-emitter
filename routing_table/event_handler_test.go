package routing_table_test

import (
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table/fakes"
	"github.com/cloudfoundry-incubator/tcp-emitter/tcp_routes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventHandler", func() {
	var (
		fakeRoutingTable *fakes.FakeRoutingTable
		fakeEmitter      *fakes.FakeEmitter
		eventHandler     routing_table.EventHandler
	)

	BeforeEach(func() {
		fakeRoutingTable = new(fakes.FakeRoutingTable)
		fakeEmitter = new(fakes.FakeEmitter)
		eventHandler = routing_table.NewEventHandler(logger, fakeRoutingTable, fakeEmitter)
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
				eventHandler.HandleDesiredCreate(desiredLRP)
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
				eventHandler.HandleDesiredUpdate(desiredLRP, after)
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
				eventHandler.HandleDesiredDelete(desiredLRP)
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
				eventHandler.HandleActualCreate(actualLRP)
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
				eventHandler.HandleActualUpdate(actualLRP, afterLRP)
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
				eventHandler.HandleActualDelete(actualLRP)
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
})
