package nbdkit

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNbdkit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nbdkit Suite")
}
