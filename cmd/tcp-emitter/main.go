package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cloudfoundry-incubator/bbs"
	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/cf_http"
	"github.com/cloudfoundry-incubator/consuladapter"
	"github.com/cloudfoundry-incubator/locket"
	"github.com/cloudfoundry-incubator/routing-api"
	"github.com/cloudfoundry-incubator/tcp-emitter/config"
	"github.com/cloudfoundry-incubator/tcp-emitter/routing_table"
	"github.com/cloudfoundry-incubator/tcp-emitter/syncer"
	"github.com/cloudfoundry-incubator/tcp-emitter/watcher"
	token_fetcher "github.com/cloudfoundry-incubator/uaa-token-fetcher"
	"github.com/cloudfoundry/dropsonde"
	"github.com/nu7hatch/gouuid"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

const (
	TCP_EMITTER_LOCK_PATH = "tcp_emitter_lock"
)

var bbsAddress = flag.String(
	"bbsAddress",
	"",
	"URL of BBS Server",
)

var communicationTimeout = flag.Duration(
	"communicationTimeout",
	10*time.Second,
	"Timeout applied to all HTTP requests.",
)

var syncInterval = flag.Duration(
	"syncInterval",
	time.Minute,
	"The interval between syncs of the routing table from bbs.",
)

var bbsCACert = flag.String(
	"bbsCACert",
	"",
	"path to certificate authority cert used for mutually authenticated TLS BBS communication",
)

var bbsClientCert = flag.String(
	"bbsClientCert",
	"",
	"path to client cert used for mutually authenticated TLS BBS communication",
)

var bbsClientKey = flag.String(
	"bbsClientKey",
	"",
	"path to client key used for mutually authenticated TLS BBS communication",
)

var configFile = flag.String(
	"config",
	"/var/vcap/jobs/tcp_emitter/config/tcp_emitter.yml",
	"The TCP emitter yml config.",
)

var consulCluster = flag.String(
	"consulCluster",
	"",
	"comma-separated list of consul server URLs (scheme://ip:port)",
)

var lockTTL = flag.Duration(
	"lockTTL",
	locket.LockTTL,
	"TTL for service lock",
)

var lockRetryInterval = flag.Duration(
	"lockRetryInterval",
	locket.RetryInterval,
	"interval to wait before retrying a failed lock acquisition",
)

var sessionName = flag.String(
	"sessionName",
	"tcp-emitter",
	"consul session name",
)

const (
	dropsondeDestination = "localhost:3457"
	dropsondeOrigin      = "tcp_emitter"
)

func main() {
	cf_debug_server.AddFlags(flag.CommandLine)
	cf_lager.AddFlags(flag.CommandLine)
	flag.Parse()

	cf_http.Initialize(*communicationTimeout)

	logger, reconfigurableSink := cf_lager.New("tcp-emitter")
	clock := clock.NewClock()

	initializeDropsonde(logger)

	bbsClient, err := bbs.NewSecureClient(*bbsAddress, *bbsCACert, *bbsClientCert, *bbsClientKey)
	if err != nil {
		logger.Error("failed-to-configure-bbs-client", err)
		os.Exit(1)
	}

	cfg, err := config.New(*configFile)
	if err != nil {
		logger.Error("failed-to-unmarshal-config-file", err)
		os.Exit(1)
	}
	tokenFetcher := token_fetcher.NewTokenFetcher(&cfg.OAuth)
	routingApiAddress := fmt.Sprintf("%s:%d", cfg.RoutingApi.Uri, cfg.RoutingApi.Port)
	logger.Debug("creating-routing-api-client", lager.Data{"api-location": routingApiAddress})
	routingApiClient := routing_api.NewClient(routingApiAddress)

	emitter := routing_table.NewEmitter(logger, routingApiClient, tokenFetcher)
	routingTable := routing_table.NewTable(logger, nil)
	routingTableHandler := routing_table.NewRoutingTableHandler(logger, routingTable, emitter, bbsClient)
	syncChannel := make(chan struct{})
	syncRunner := syncer.New(clock, *syncInterval, syncChannel, logger)
	watcher := watcher.NewWatcher(bbsClient, clock, routingTableHandler, syncChannel, logger)

	lockMaintainer := initializeLockMaintainer(logger, *consulCluster, *sessionName,
		*lockTTL, *lockRetryInterval, clock)

	members := grouper.Members{
		{"lock-maintainer", lockMaintainer},
		{"watcher", watcher},
		{"syncer", syncRunner},
	}

	if dbgAddr := cf_debug_server.DebugAddress(flag.CommandLine); dbgAddr != "" {
		members = append(grouper.Members{
			{"debug-server", cf_debug_server.Runner(dbgAddr, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)

	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

	err = <-monitor.Wait()
	if err != nil {
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}

	logger.Info("exited")
}

func initializeDropsonde(logger lager.Logger) {
	err := dropsonde.Initialize(dropsondeDestination, dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeLockMaintainer(
	logger lager.Logger,
	consulCluster, sessionName string,
	lockTTL, lockRetryInterval time.Duration,
	clock clock.Clock,
) ifrit.Runner {
	client, err := consuladapter.NewClient(consulCluster)
	if err != nil {
		logger.Fatal("new-client-failed", err)
	}
	sessionMgr := consuladapter.NewSessionManager(client)
	consulSession, err := consuladapter.NewSession(sessionName, lockTTL, client, sessionMgr)
	if err != nil {
		logger.Fatal("consul-session-failed", err)
	}

	return newLockRunner(logger, consulSession, clock, lockRetryInterval)
}

func newLockRunner(
	logger lager.Logger,
	consulSession *consuladapter.Session,
	clock clock.Clock,
	lockRetryInterval time.Duration) ifrit.Runner {
	lockSchemaPath := locket.LockSchemaPath(TCP_EMITTER_LOCK_PATH)

	tcpEmitterUUID, err := uuid.NewV4()
	if err != nil {
		logger.Fatal("Couldn't generate tcp Emitter UUID", err)
	}
	tcpEmitterId := []byte(tcpEmitterUUID.String())

	return locket.NewLock(consulSession, lockSchemaPath,
		tcpEmitterId, clock, lockRetryInterval, logger)
}
