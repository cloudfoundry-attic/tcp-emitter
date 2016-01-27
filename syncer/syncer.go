package syncer

import (
	"os"
	"time"

	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

type Syncer struct {
	clock        clock.Clock
	syncInterval time.Duration
	syncChannel  chan struct{}
	logger       lager.Logger
}

func New(
	clock clock.Clock,
	syncInterval time.Duration,
	syncChannel chan struct{},
	logger lager.Logger,
) *Syncer {
	return &Syncer{
		clock:        clock,
		syncInterval: syncInterval,
		syncChannel:  syncChannel,

		logger: logger.Session("syncer"),
	}
}

func (s *Syncer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	s.logger.Info("starting")
	close(ready)
	s.logger.Info("started")

	s.logger.Debug("starting-initial-sync")
	s.sync()

	//now keep emitting at the desired interval, syncing with etcd every syncInterval
	syncTicker := s.clock.NewTicker(s.syncInterval)

	for {
		select {
		case <-syncTicker.C():
			s.logger.Info("syncing")
			s.sync()
		case <-signals:
			s.logger.Info("stopping")
			syncTicker.Stop()
			return nil
		}
	}
}

func (s *Syncer) sync() {
	select {
	case s.syncChannel <- struct{}{}:
	default:
		s.logger.Debug("sync-already-in-progress")
	}
}
