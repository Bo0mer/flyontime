package flyontime_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFlyontime(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flyontime Suite")
}
