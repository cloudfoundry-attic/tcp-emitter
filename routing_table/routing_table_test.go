package routing_table_test

import (
	"encoding/json"

	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/routing-info/tcp_routes"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

type testRoutingTable struct {
	entries map[routing_table.RoutingKey]routing_table.RoutableEndpoints
}

func (t *testRoutingTable) RouteCount() int {
	return 0
}

func (t *testRoutingTable) AddRoutes(desiredLRP *models.DesiredLRP) routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}

func (t *testRoutingTable) UpdateRoutes(before, after *models.DesiredLRP) routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}

func (t *testRoutingTable) RemoveRoutes(desiredLRP *models.DesiredLRP) routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}

func (t *testRoutingTable) AddEndpoint(actualLRP *models.ActualLRPGroup) routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}

func (t *testRoutingTable) RemoveEndpoint(actualLRP *models.ActualLRPGroup) routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}

func (t *testRoutingTable) Swap(table routing_table.RoutingTable) routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}

func (t *testRoutingTable) GetRoutingEvents() routing_table.RoutingEvents {
	return routing_table.RoutingEvents{}
}

var _ = Describe("RoutingTable", func() {

	var (
		routingTable    routing_table.RoutingTable
		modificationTag *models.ModificationTag
		tcpRoutes       tcp_routes.TCPRoutes
	)

	getDesiredLRP := func(processGuid, logGuid string,
		tcpRoutes tcp_routes.TCPRoutes, modificationTag *models.ModificationTag) *models.DesiredLRP {
		var desiredLRP models.DesiredLRP
		portMap := map[uint32]struct{}{}
		for _, tcpRoute := range tcpRoutes {
			portMap[tcpRoute.ContainerPort] = struct{}{}
		}

		ports := []uint32{}
		for k, _ := range portMap {
			ports = append(ports, k)
		}

		desiredLRP.ProcessGuid = processGuid
		desiredLRP.Ports = ports
		desiredLRP.LogGuid = logGuid
		desiredLRP.ModificationTag = modificationTag
		desiredLRP.Routes = tcpRoutes.RoutingInfo()

		// add 'diego-ssh' data for testing sanitize
		routingInfo := json.RawMessage([]byte(`{ "private_key": "fake-key" }`))
		(*desiredLRP.Routes)["diego-sshd"] = &routingInfo

		return &desiredLRP
	}

	getActualLRP := func(processGuid, instanceGuid, hostAddress string,
		hostPort, containerPort uint32, evacuating bool,
		modificationTag *models.ModificationTag) *models.ActualLRPGroup {
		if evacuating {
			return &models.ActualLRPGroup{
				Instance: nil,
				Evacuating: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						hostAddress,
						models.NewPortMapping(hostPort, containerPort),
					),
					State:           models.ActualLRPStateRunning,
					ModificationTag: *modificationTag,
				},
			}
		} else {
			return &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						hostAddress,
						models.NewPortMapping(hostPort, containerPort),
					),
					State:           models.ActualLRPStateRunning,
					ModificationTag: *modificationTag,
				},
				Evacuating: nil,
			}
		}
	}

	BeforeEach(func() {
		tcpRoutes = tcp_routes.TCPRoutes{
			tcp_routes.TCPRoute{
				ExternalPort:  61000,
				ContainerPort: 5222,
			},
		}
	})

	Context("when no entry exist for route", func() {
		BeforeEach(func() {
			routingTable = routing_table.NewTable(logger, nil)
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 0}
		})

		Describe("AddRoutes", func() {
			It("emits nothing", func() {
				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents := routingTable.AddRoutes(desiredLRP)
				Expect(routingEvents).To(HaveLen(0))
			})

			It("does not emit any sensitive information", func() {
				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents := routingTable.AddRoutes(desiredLRP)
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
				Expect(routingEvents).To(HaveLen(0))
			})
		})

		Describe("UpdateRoutes", func() {
			It("emits nothing", func() {
				beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
				afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
				routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
				Expect(routingEvents).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
				afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
				routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
				Expect(routingEvents).To(HaveLen(0))
			})
		})

		Describe("RemoveRoutes", func() {
			It("emits nothing", func() {
				desiredLRP := getDesiredLRP("process-guid-10", "log-guid-10", tcpRoutes, modificationTag)
				routingEvents := routingTable.RemoveRoutes(desiredLRP)
				Expect(routingEvents).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				desiredLRP := getDesiredLRP("process-guid-10", "log-guid-10", tcpRoutes, modificationTag)
				routingEvents := routingTable.RemoveRoutes(desiredLRP)
				Expect(routingEvents).To(HaveLen(0))
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
			})
		})

		Describe("AddEndpoint", func() {
			It("emits nothing", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.AddEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.AddEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
			})
		})

		Describe("RemoveEndpoint", func() {
			It("emits nothing", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.RemoveEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, modificationTag)
				routingEvents := routingTable.RemoveEndpoint(actualLRP)
				Expect(routingEvents).To(HaveLen(0))
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
			})
		})

		Describe("Swap", func() {
			var (
				tempRoutingTable routing_table.RoutingTable
				key              routing_table.RoutingKey
				endpoints        map[routing_table.EndpointKey]routing_table.Endpoint
				modificationTag  *models.ModificationTag
				logGuid          string
			)

			BeforeEach(func() {
				logGuid = "log-guid-1"
				externalEndpoints := routing_table.ExternalEndpointInfos{
					routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
				}
				key = routing_table.NewRoutingKey("process-guid-1", 5222)
				modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
				endpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
					routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
						"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
					routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
						"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
				}

				tempRoutingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
					key: routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
				})
			})

			It("emits routing events for new routes", func() {
				Expect(routingTable.RouteCount()).Should(Equal(0))
				routingEvents := routingTable.Swap(tempRoutingTable)
				Expect(routingTable.RouteCount()).Should(Equal(1))
				Expect(routingEvents).To(HaveLen(1))
				routingEvent := routingEvents[0]
				Expect(routingEvent.Key).Should(Equal(key))
				Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
				externalInfo := routing_table.ExternalEndpointInfos{
					routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
				}
				expectedEntry := routing_table.NewRoutableEndpoints(
					externalInfo, endpoints, logGuid, modificationTag)
				Expect(routingEvent.Entry).Should(Equal(expectedEntry))
			})
		})

		Describe("GetRoutingEvents", func() {
			It("returns empty routing events", func() {
				routingEvents := routingTable.GetRoutingEvents()
				Expect(routingEvents).To(HaveLen(0))
			})
		})
	})

	Context("when there exists an entry for route", func() {
		var (
			endpoints         map[routing_table.EndpointKey]routing_table.Endpoint
			key               routing_table.RoutingKey
			logGuid           string
			externalEndpoints routing_table.ExternalEndpointInfos
		)

		BeforeEach(func() {
			logGuid = "log-guid-1"
			externalEndpoints = routing_table.ExternalEndpointInfos{
				routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
			}
			key = routing_table.NewRoutingKey("process-guid-1", 5222)
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
			endpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
				routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
					"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
				routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
					"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
			}
		})

		Describe("AddRoutes", func() {
			BeforeEach(func() {
				routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
					key: routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
				})
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})

			Context("existing external port changes", func() {
				var (
					newTcpRoutes tcp_routes.TCPRoutes
				)
				BeforeEach(func() {
					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
					}
				})

				It("emits routing event with modified external port", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
					routingEvents := routingTable.AddRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(2))

					routingEvent := routingEvents[1]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteUnregistrationEvent))
					unregistrationExpectedEntry := routing_table.NewRoutableEndpoints(
						externalEndpoints, endpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))

					externalInfo := []routing_table.ExternalEndpointInfo{
						routing_table.NewExternalEndpointInfo("router-group-guid", 61001),
					}
					routingEvent = routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
					registrationExpectedEntry := routing_table.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, newModificationTag)
					Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("older modification tag", func() {
					It("emits nothing", func() {
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingEvents := routingTable.AddRoutes(desiredLRP)
						Expect(routingEvents).To(HaveLen(0))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})
			})

			Context("new external port is added", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61002,
							ContainerPort:   5222,
						},
					}
				})

				It("emits routing event with both external ports", func() {
					modificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingEvents := routingTable.AddRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(1))
					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
					externalInfo := []routing_table.ExternalEndpointInfo{
						routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
						routing_table.NewExternalEndpointInfo("router-group-guid", 61001),
						routing_table.NewExternalEndpointInfo("router-group-guid", 61002),
					}
					expectedEntry := routing_table.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("older modification tag", func() {
					It("emits nothing", func() {
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingEvents := routingTable.AddRoutes(desiredLRP)
						Expect(routingEvents).To(HaveLen(0))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})
			})

			Context("multiple external port added and multiple existing external ports deleted", func() {
				var (
					newTcpRoutes tcp_routes.TCPRoutes
				)
				BeforeEach(func() {
					externalEndpoints = routing_table.ExternalEndpointInfos{
						routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
						routing_table.NewExternalEndpointInfo("router-group-guid", 61001),
					}
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))

					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61002,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61003,
							ContainerPort:   5222,
						},
					}
				})

				It("emits routing event with both external ports", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
					routingEvents := routingTable.AddRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(2))

					routingEvent := routingEvents[1]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteUnregistrationEvent))
					unregistrationExpectedEntry := routing_table.NewRoutableEndpoints(
						externalEndpoints, endpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))

					externalInfo := []routing_table.ExternalEndpointInfo{
						routing_table.NewExternalEndpointInfo("router-group-guid", 61002),
						routing_table.NewExternalEndpointInfo("router-group-guid", 61003),
					}
					routingEvent = routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
					registrationExpectedEntry := routing_table.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, newModificationTag)
					Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("older modification tag", func() {
					It("emits nothing", func() {
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingEvents := routingTable.AddRoutes(desiredLRP)
						Expect(routingEvents).To(HaveLen(0))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})
			})

			Context("no changes to external port", func() {
				It("emits nothing", func() {
					tag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, tag)
					routingEvents := routingTable.AddRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(0))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})
			})

			Context("when two disjoint (external port, container port) pairs are given", func() {
				var endpoints2 map[routing_table.EndpointKey]routing_table.Endpoint
				var key2 routing_table.RoutingKey

				createdExpectedEvents := func(newModificationTag *models.ModificationTag) []routing_table.RoutingEvent {
					externalInfo1 := []routing_table.ExternalEndpointInfo{
						routing_table.NewExternalEndpointInfo("router-group-guid", 61001),
					}
					expectedEntry1 := routing_table.NewRoutableEndpoints(
						externalInfo1, endpoints, logGuid, newModificationTag)

					externalInfo2 := []routing_table.ExternalEndpointInfo{
						routing_table.NewExternalEndpointInfo("router-group-guid", 61002),
					}
					expectedEntry2 := routing_table.NewRoutableEndpoints(
						externalInfo2, endpoints2, logGuid, newModificationTag)

					externalInfo3 := []routing_table.ExternalEndpointInfo{
						routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
					}
					expectedEntry3 := routing_table.NewRoutableEndpoints(
						externalInfo3, endpoints, logGuid, modificationTag)

					return []routing_table.RoutingEvent{
						routing_table.RoutingEvent{
							EventType: routing_table.RouteRegistrationEvent,
							Key:       key2,
							Entry:     expectedEntry2,
						}, routing_table.RoutingEvent{
							EventType: routing_table.RouteRegistrationEvent,
							Key:       key,
							Entry:     expectedEntry1,
						}, routing_table.RoutingEvent{
							EventType: routing_table.RouteUnregistrationEvent,
							Key:       key,
							Entry:     expectedEntry3,
						},
					}
				}

				BeforeEach(func() {
					key2 = routing_table.NewRoutingKey("process-guid-1", 5223)
					endpoints2 = map[routing_table.EndpointKey]routing_table.Endpoint{
						routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
							"instance-guid-1", false, "some-ip-1", 63004, 5223, modificationTag),
					}
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key:  routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
						key2: routing_table.NewRoutableEndpoints(nil, endpoints2, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(2))

					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61002,
							ContainerPort:   5223,
						},
					}
				})

				It("emits two separate registration events with no overlap", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
					routingEvents := routingTable.AddRoutes(desiredLRP)

					expectedEvents := createdExpectedEvents(newModificationTag)

					// Two registration and one unregistration events
					Expect(routingEvents).To(HaveLen(3))
					Expect(routingEvents).To(ConsistOf(expectedEvents))
					Expect(routingTable.RouteCount()).Should(Equal(2))
				})
			})

			Context("when container ports don't match", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							ExternalPort:  61000,
							ContainerPort: 5223,
						},
					}
				})

				It("emits nothing", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newTag)
					routingEvents := routingTable.AddRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(0))
					Expect(routingTable.RouteCount()).Should(Equal(2))
				})
			})
		})

		Describe("UpdateRoutes", func() {
			var (
				oldTcpRoutes       tcp_routes.TCPRoutes
				newTcpRoutes       tcp_routes.TCPRoutes
				newModificationTag *models.ModificationTag
			)

			BeforeEach(func() {
				newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
				routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
					key: routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
				})
				Expect(routingTable.RouteCount()).Should(Equal(1))
				oldTcpRoutes = tcp_routes.TCPRoutes{
					tcp_routes.TCPRoute{
						RouterGroupGuid: "router-group-guid",
						ExternalPort:    61000,
						ContainerPort:   5222,
					},
				}
			})

			Context("when there is no change in container ports", func() {
				BeforeEach(func() {
					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
					}
				})

				Context("when there is change in external port", func() {
					It("emits registration and unregistration events", func() {
						beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
						routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
						Expect(routingEvents).To(HaveLen(2))

						externalInfo := []routing_table.ExternalEndpointInfo{
							routing_table.NewExternalEndpointInfo("router-group-guid", 61001),
						}
						routingEvent := routingEvents[0]
						Expect(routingEvent.Key).Should(Equal(key))
						Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
						registrationExpectedEntry := routing_table.NewRoutableEndpoints(
							externalInfo, endpoints, logGuid, newModificationTag)
						Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

						routingEvent = routingEvents[1]
						Expect(routingEvent.Key).Should(Equal(key))
						Expect(routingEvent.EventType).Should(Equal(routing_table.RouteUnregistrationEvent))
						unregistrationExpectedEntry := routing_table.NewRoutableEndpoints(
							externalEndpoints, endpoints, logGuid, modificationTag)
						Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))

						Expect(routingTable.RouteCount()).Should(Equal(1))
					})

					Context("with older modification tag", func() {
						It("emits nothing", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(0))
							Expect(routingTable.RouteCount()).Should(Equal(1))
						})
					})
				})

				Context("when there is no change in external port", func() {
					It("emits nothing", func() {
						beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, newModificationTag)
						routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
						Expect(routingEvents).To(HaveLen(0))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})
			})

			Context("when new container port is added", func() {
				Context("when mapped to new external port", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5222,
							},
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61001,
								ContainerPort:   5223,
							},
						}
					})

					Context("no backends for new container port", func() {
						It("emits no routing events and adds to routing table entry", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(0))
							Expect(routingTable.RouteCount()).Should(Equal(2))
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount + 1))
							})
						})
					})

					Context("existing backends for new container port", func() {
						var (
							newKey       routing_table.RoutingKey
							newEndpoints map[routing_table.EndpointKey]routing_table.Endpoint
						)
						BeforeEach(func() {
							newKey = routing_table.NewRoutingKey("process-guid-1", 5223)
							newEndpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
								routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 62006, 5223, modificationTag),
							}
							routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
								key:    routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
								newKey: routing_table.NewRoutableEndpoints(routing_table.ExternalEndpointInfos{}, newEndpoints, logGuid, modificationTag),
							})
						})

						It("emits registration events for new container port", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(1))

							externalInfo := []routing_table.ExternalEndpointInfo{
								routing_table.NewExternalEndpointInfo("router-group-guid", 61001),
							}
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(newKey))
							Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
							registrationExpectedEntry := routing_table.NewRoutableEndpoints(
								externalInfo, newEndpoints, logGuid, newModificationTag)
							Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
						})

						Context("with older modification tag", func() {
							It("emits nothing", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
							})
						})
					})

				})

				Context("when mapped to existing external port", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5222,
							},
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5223,
							},
						}
					})

					Context("no backends for new container port", func() {
						It("emits no routing events and adds to routing table entry", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(0))
							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount + 1))
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount + 1))
							})
						})
					})

					Context("existing backends for new container port", func() {
						var (
							newKey       routing_table.RoutingKey
							newEndpoints map[routing_table.EndpointKey]routing_table.Endpoint
						)
						BeforeEach(func() {
							newKey = routing_table.NewRoutingKey("process-guid-1", 5223)
							newEndpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
								routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 62006, 5223, modificationTag),
							}
							routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
								key:    routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
								newKey: routing_table.NewRoutableEndpoints(routing_table.ExternalEndpointInfos{}, newEndpoints, logGuid, modificationTag),
							})
						})

						It("emits registration events for new container port", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(1))

							externalInfo := []routing_table.ExternalEndpointInfo{
								routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
							}
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(newKey))
							Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
							registrationExpectedEntry := routing_table.NewRoutableEndpoints(
								externalInfo, newEndpoints, logGuid, newModificationTag)
							Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
						})

						Context("with older modification tag", func() {
							It("emits nothing", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
							})
						})
					})
				})
			})

			Context("when existing container port is removed", func() {

				Context("when there are no routes left", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{}
					})
					It("emits only unregistration events", func() {
						beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
						currentRoutesCount := routingTable.RouteCount()
						routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
						Expect(routingEvents).To(HaveLen(1))

						routingEvent := routingEvents[0]
						Expect(routingEvent.Key).Should(Equal(key))
						Expect(routingEvent.EventType).Should(Equal(routing_table.RouteUnregistrationEvent))
						unregistrationExpectedEntry := routing_table.NewRoutableEndpoints(
							externalEndpoints, endpoints, logGuid, modificationTag)
						Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))
						Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount - 1))
					})

					Context("with older modification tag", func() {
						It("emits nothing", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(0))
							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
						})
					})
				})

				Context("when container port is switched", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5223,
							},
						}
					})

					Context("no backends for new container port", func() {
						It("emits no routing events and adds to routing table entry", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(1))

							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(routing_table.RouteUnregistrationEvent))
							unregistrationExpectedEntry := routing_table.NewRoutableEndpoints(
								externalEndpoints, endpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))

							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount + 1))
							})
						})
					})

					Context("existing backends for new container port", func() {
						var (
							newKey       routing_table.RoutingKey
							newEndpoints map[routing_table.EndpointKey]routing_table.Endpoint
						)
						BeforeEach(func() {
							newKey = routing_table.NewRoutingKey("process-guid-1", 5223)
							newEndpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
								routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 62006, 5223, modificationTag),
							}
							routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
								key:    routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
								newKey: routing_table.NewRoutableEndpoints(routing_table.ExternalEndpointInfos{}, newEndpoints, logGuid, modificationTag),
							})
						})

						It("emits registration events for new container port", func() {
							beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.RouteCount()
							routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
							Expect(routingEvents).To(HaveLen(2))

							externalInfo := []routing_table.ExternalEndpointInfo{
								routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
							}
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(newKey))
							Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
							registrationExpectedEntry := routing_table.NewRoutableEndpoints(
								externalInfo, newEndpoints, logGuid, newModificationTag)
							Expect(routingEvent.Entry).Should(Equal(registrationExpectedEntry))

							routingEvent = routingEvents[1]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(routing_table.RouteUnregistrationEvent))
							unregistrationExpectedEntry := routing_table.NewRoutableEndpoints(
								externalInfo, endpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry).Should(Equal(unregistrationExpectedEntry))

							Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount - 1))
						})

						Context("with older modification tag", func() {
							It("emits nothing", func() {
								beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", oldTcpRoutes, modificationTag)
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.RouteCount()
								routingEvents := routingTable.UpdateRoutes(beforeLRP, afterLRP)
								Expect(routingEvents).To(HaveLen(0))
								Expect(routingTable.RouteCount()).Should(Equal(currentRoutesCount))
							})
						})
					})
				})
			})
		})

		Describe("RemoveRoutes", func() {
			Context("when entry does not have endpoints", func() {
				BeforeEach(func() {
					emptyEndpoints := make(map[routing_table.EndpointKey]routing_table.Endpoint)
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoints, emptyEndpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				It("emits nothing", func() {
					modificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingEvents := routingTable.RemoveRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(0))
				})
			})

			Context("when entry does have endpoints", func() {
				BeforeEach(func() {
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				It("emits unregistration routing events", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
					routingEvents := routingTable.RemoveRoutes(desiredLRP)
					Expect(routingEvents).To(HaveLen(1))
					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteUnregistrationEvent))
					expectedEntry := routing_table.NewRoutableEndpoints(
						externalEndpoints, endpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(0))
				})
			})
		})

		Describe("AddEndpoint", func() {

			Context("with no existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoints, nil, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				It("emits routing events", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 1}
					actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, newTag)
					routingEvents := routingTable.AddEndpoint(actualLRP)
					Expect(routingEvents).To(HaveLen(1))
					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))

					expectedEndpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
						routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
							"instance-guid-1", false, "some-ip-1", 61104, 5222, newTag),
					}

					expectedEntry := routing_table.NewRoutableEndpoints(
						externalEndpoints, expectedEndpoints, logGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})
			})

			Context("with existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("with different instance guid", func() {
					It("emits routing events", func() {
						newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						actualLRP := getActualLRP("process-guid-1", "instance-guid-3", "some-ip-3", 61104, 5222, false, newTag)
						routingEvents := routingTable.AddEndpoint(actualLRP)
						Expect(routingEvents).To(HaveLen(1))
						routingEvent := routingEvents[0]
						Expect(routingEvent.Key).Should(Equal(key))
						Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))

						expectedEndpoints := map[routing_table.EndpointKey]routing_table.Endpoint{}
						for k, v := range endpoints {
							expectedEndpoints[k] = v
						}
						expectedEndpoints[routing_table.NewEndpointKey("instance-guid-3", false)] =
							routing_table.NewEndpoint(
								"instance-guid-3", false, "some-ip-3", 61104, 5222, newTag)
						expectedEntry := routing_table.NewRoutableEndpoints(
							externalEndpoints, expectedEndpoints, logGuid, modificationTag)
						Expect(routingEvent.Entry.Endpoints).Should(HaveLen(3))
						Expect(routingEvent.Entry).Should(Equal(expectedEntry))
						Expect(routingTable.RouteCount()).Should(Equal(1))
					})
				})

				Context("with same instance guid", func() {
					Context("newer modification tag", func() {
						It("emits routing events", func() {
							newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61105, 5222, false, newTag)
							routingEvents := routingTable.AddEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(1))
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))

							expectedEndpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
								routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 61105, 5222, newTag),
								routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
									"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
							}
							expectedEntry := routing_table.NewRoutableEndpoints(
								externalEndpoints, expectedEndpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry.Endpoints).Should(HaveLen(2))
							Expect(routingEvent.Entry).Should(Equal(expectedEntry))
							Expect(routingTable.RouteCount()).Should(Equal(1))
						})
					})

					Context("older modification tag", func() {
						It("emits nothing", func() {
							olderTag := &models.ModificationTag{Epoch: "abc", Index: 0}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61105, 5222, false, olderTag)
							routingEvents := routingTable.AddEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(0))
						})
					})
				})

			})
		})

		Describe("RemoveEndpoint", func() {

			Context("with no existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoints, nil, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				It("emits nothing", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 1}
					actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 61104, 5222, false, newTag)
					routingEvents := routingTable.RemoveEndpoint(actualLRP)
					Expect(routingEvents).To(HaveLen(0))
				})
			})

			Context("with existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
					})
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})

				Context("with instance guid not present in existing endpoints", func() {
					It("emits nothing", func() {
						newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						actualLRP := getActualLRP("process-guid-1", "instance-guid-3", "some-ip-3", 62004, 5222, false, newTag)
						routingEvents := routingTable.RemoveEndpoint(actualLRP)
						Expect(routingEvents).To(HaveLen(0))
					})
				})

				Context("with same instance guid", func() {
					Context("newer modification tag", func() {
						It("emits routing events", func() {
							newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 62004, 5222, false, newTag)
							routingEvents := routingTable.RemoveEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(1))
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(routing_table.RouteUnregistrationEvent))

							expectedEndpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
								routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
							}
							expectedEntry := routing_table.NewRoutableEndpoints(
								externalEndpoints, expectedEndpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry.Endpoints).Should(HaveLen(1))
							Expect(routingEvent.Entry).Should(Equal(expectedEntry))
							Expect(routingTable.RouteCount()).Should(Equal(1))
						})
					})

					Context("same modification tag", func() {
						It("emits routing events", func() {
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 62004, 5222, false, modificationTag)
							routingEvents := routingTable.RemoveEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(1))
							routingEvent := routingEvents[0]
							Expect(routingEvent.Key).Should(Equal(key))
							Expect(routingEvent.EventType).Should(Equal(routing_table.RouteUnregistrationEvent))

							expectedEndpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
								routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
									"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
							}
							expectedEntry := routing_table.NewRoutableEndpoints(
								externalEndpoints, expectedEndpoints, logGuid, modificationTag)
							Expect(routingEvent.Entry.Endpoints).Should(HaveLen(1))
							Expect(routingEvent.Entry).Should(Equal(expectedEntry))
							Expect(routingTable.RouteCount()).Should(Equal(1))
						})
					})

					Context("older modification tag", func() {
						It("emits nothing", func() {
							olderTag := &models.ModificationTag{Epoch: "abc", Index: 0}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", 62004, 5222, false, olderTag)
							routingEvents := routingTable.RemoveEndpoint(actualLRP)
							Expect(routingEvents).To(HaveLen(0))
						})
					})
				})

			})
		})

		Describe("GetRoutingEvents", func() {
			BeforeEach(func() {
				routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
					key: routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
				})
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})

			It("returns routing events for entries in routing table", func() {
				routingEvents := routingTable.GetRoutingEvents()
				Expect(routingEvents).To(HaveLen(1))
				routingEvent := routingEvents[0]
				Expect(routingEvent.Key).Should(Equal(key))
				Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
				externalInfo := []routing_table.ExternalEndpointInfo{
					routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
				}
				expectedEntry := routing_table.NewRoutableEndpoints(
					externalInfo, endpoints, logGuid, modificationTag)
				Expect(routingEvent.Entry).Should(Equal(expectedEntry))
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})
		})

		Describe("Swap", func() {
			var (
				tempRoutingTable   routing_table.RoutingTable
				key                routing_table.RoutingKey
				existingKey        routing_table.RoutingKey
				endpoints          map[routing_table.EndpointKey]routing_table.Endpoint
				existingEndpoints  map[routing_table.EndpointKey]routing_table.Endpoint
				modificationTag    *models.ModificationTag
				newModificationTag *models.ModificationTag
				logGuid            string
				existingLogGuid    string
			)

			BeforeEach(func() {
				existingLogGuid = "log-guid-1"
				existingExternalEndpoint := routing_table.ExternalEndpointInfos{
					routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
				}
				existingKey = routing_table.NewRoutingKey("process-guid-1", 5222)
				modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
				existingEndpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
					routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
						"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
					routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
						"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
				}
				routingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
					existingKey: routing_table.NewRoutableEndpoints(existingExternalEndpoint, existingEndpoints, existingLogGuid, modificationTag),
				})
				Expect(routingTable.RouteCount()).Should(Equal(1))
			})

			Context("when adding a new routing key (process-guid, container-port)", func() {

				BeforeEach(func() {
					logGuid = "log-guid-2"
					externalEndpoints := routing_table.ExternalEndpointInfos{
						routing_table.NewExternalEndpointInfo("router-group-guid", 62000),
					}
					key = routing_table.NewRoutingKey("process-guid-2", 6379)
					newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					endpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
						routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
							"instance-guid-1", false, "some-ip-3", 63004, 6379, newModificationTag),
						routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
							"instance-guid-2", false, "some-ip-4", 63004, 6379, newModificationTag),
					}
					tempRoutingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						key: routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, newModificationTag),
					})
					Expect(tempRoutingTable.RouteCount()).Should(Equal(1))
				})

				It("overwrites the existing entries and emits registration and unregistration routing events", func() {
					routingEvents := routingTable.Swap(tempRoutingTable)
					Expect(routingEvents).To(HaveLen(2))

					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(key))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
					externalInfo := routing_table.ExternalEndpointInfos{
						routing_table.NewExternalEndpointInfo("router-group-guid", 62000),
					}
					expectedEntry := routing_table.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, newModificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))

					routingEvent = routingEvents[1]
					Expect(routingEvent.Key).Should(Equal(existingKey))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteUnregistrationEvent))
					externalInfo = routing_table.ExternalEndpointInfos{
						routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
					}
					expectedEntry = routing_table.NewRoutableEndpoints(
						externalInfo, existingEndpoints, existingLogGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})
			})

			Context("when updating an existing routing key (process-guid, container-port)", func() {

				BeforeEach(func() {
					logGuid = "log-guid-2"
					externalEndpoints := routing_table.ExternalEndpointInfos{
						routing_table.NewExternalEndpointInfo("router-group-guid", 62000),
					}
					newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					endpoints = map[routing_table.EndpointKey]routing_table.Endpoint{
						routing_table.NewEndpointKey("instance-guid-3", false): routing_table.NewEndpoint(
							"instance-guid-1", false, "some-ip-3", 63004, 5222, newModificationTag),
						routing_table.NewEndpointKey("instance-guid-4", false): routing_table.NewEndpoint(
							"instance-guid-2", false, "some-ip-4", 63004, 5222, newModificationTag),
					}
					tempRoutingTable = routing_table.NewTable(logger, map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
						existingKey: routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, newModificationTag),
					})
					Expect(tempRoutingTable.RouteCount()).Should(Equal(1))
				})

				It("overwrites the existing entries and emits registration and unregistration routing events", func() {
					routingEvents := routingTable.Swap(tempRoutingTable)
					Expect(routingEvents).To(HaveLen(2))

					routingEvent := routingEvents[0]
					Expect(routingEvent.Key).Should(Equal(existingKey))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteRegistrationEvent))
					externalInfo := routing_table.ExternalEndpointInfos{
						routing_table.NewExternalEndpointInfo("router-group-guid", 62000),
					}
					expectedEntry := routing_table.NewRoutableEndpoints(
						externalInfo, endpoints, logGuid, newModificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))

					routingEvent = routingEvents[1]
					Expect(routingEvent.Key).Should(Equal(existingKey))
					Expect(routingEvent.EventType).Should(Equal(routing_table.RouteUnregistrationEvent))
					externalInfo = routing_table.ExternalEndpointInfos{
						routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
					}
					expectedEntry = routing_table.NewRoutableEndpoints(
						externalInfo, existingEndpoints, existingLogGuid, modificationTag)
					Expect(routingEvent.Entry).Should(Equal(expectedEntry))
					Expect(routingTable.RouteCount()).Should(Equal(1))
				})
			})
		})
	})

	Describe("Swap", func() {

		var (
			tempRoutingTable testRoutingTable
		)

		BeforeEach(func() {
			routingTable = routing_table.NewTable(logger, nil)
			tempRoutingTable = testRoutingTable{}

			logGuid := "log-guid-1"
			externalEndpoints := routing_table.ExternalEndpointInfos{
				routing_table.NewExternalEndpointInfo("router-group-guid", 61000),
			}
			key := routing_table.NewRoutingKey("process-guid-1", 5222)
			modificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
			endpoints := map[routing_table.EndpointKey]routing_table.Endpoint{
				routing_table.NewEndpointKey("instance-guid-1", false): routing_table.NewEndpoint(
					"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
				routing_table.NewEndpointKey("instance-guid-2", false): routing_table.NewEndpoint(
					"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
			}

			tempRoutingTable.entries = map[routing_table.RoutingKey]routing_table.RoutableEndpoints{
				key: routing_table.NewRoutableEndpoints(externalEndpoints, endpoints, logGuid, modificationTag),
			}

		})
		Context("when the routing tables are of different type", func() {

			It("should not swap the tables", func() {
				routingEvents := routingTable.Swap(&tempRoutingTable)
				Expect(routingEvents).To(HaveLen(0))
				Expect(routingTable.RouteCount()).Should(Equal(0))
			})
		})
	})

})
