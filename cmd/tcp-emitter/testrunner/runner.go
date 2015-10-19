package testrunner

import (
	"os/exec"
	"strconv"
	"time"

	"github.com/tedsuo/ifrit/ginkgomon"
)

type Args struct {
	BBSAddress     string
	BBSClientCert  string
	BBSCACert      string
	BBSClientKey   string
	ConfigFilePath string
	SyncInterval   time.Duration

	ConsulCluster     string
	LockRetryInterval time.Duration
	SessionName       string

	RoutingApiAuthEnabled bool
}

func (args Args) ArgSlice() []string {
	return []string{
		"-bbsAddress=" + args.BBSAddress,
		"-bbsCACert=" + args.BBSCACert,
		"-bbsClientCert=" + args.BBSClientCert,
		"-bbsClientKey=" + args.BBSClientKey,
		"-config=" + args.ConfigFilePath,
		"-syncInterval=" + args.SyncInterval.String(),
		"-logLevel=debug",
		"-lockRetryInterval", "1s",
		"-consulCluster", args.ConsulCluster,
		"-sessionName", args.SessionName,
		"-routingApiAuthEnabled=" + strconv.FormatBool(args.RoutingApiAuthEnabled),
	}
}

func New(binPath string, args Args) *ginkgomon.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Name:              "tcp-emitter",
		AnsiColorCode:     "1;95m",
		StartCheck:        "tcp-emitter.started",
		StartCheckTimeout: 10 * time.Second,
		Command:           exec.Command(binPath, args.ArgSlice()...),
	})
}
