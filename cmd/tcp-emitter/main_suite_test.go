package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTcpEmitter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TcpEmitter Suite")
}
