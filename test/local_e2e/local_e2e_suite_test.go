package local_e2e_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLocalE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LocalE2e Suite")
}
