package emitter_test

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var logger lager.Logger

var _ = BeforeEach(func() {
	logger = lagertest.NewTestLogger("test")
})

func TestEmitter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Emitter Suite")
}
