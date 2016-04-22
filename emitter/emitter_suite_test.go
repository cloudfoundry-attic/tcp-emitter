package emitter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

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