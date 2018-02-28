package flyontime_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFlyontime(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flyontime Suite")
}

var durationScaleFactor time.Duration = 1

var _ = BeforeSuite(func() {
	if f := os.Getenv("DURATION_SCALE_FACTOR"); f != "" {
		d, err := strconv.Atoi(f)
		if err != nil {
			return
		}
		durationScaleFactor = time.Duration(d)
	}
})
