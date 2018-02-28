package flyontime_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/Bo0mer/flyontime/pkg/flyontime"
	"github.com/Bo0mer/flyontime/pkg/flyontime/flyontimefakes"
)

var _ = Describe("Pilot", func() {
	Describe("FinishedBuilds", func() {
		var client *flyontimefakes.FakeConcourseClient
		var team *flyontimefakes.FakeTeam

		var pilot *AutoPilot
		var builds <-chan atc.Build
		var stop context.CancelFunc

		BeforeEach(func() {
			client = new(flyontimefakes.FakeConcourseClient)
			team = new(flyontimefakes.FakeTeam)
		})

		AfterEach(func() {
			if stop != nil {
				stop()
			}
		})

		JustBeforeEach(func() {
			pilot = &AutoPilot{
				Client:       client,
				Team:         team,
				Logger:       lager.NewLogger("test"),
				PollInterval: 5 * time.Millisecond,
			}
			ctx, cancel := context.WithCancel(context.Background())
			stop = cancel
			builds = pilot.FinishedBuilds(ctx)
		})

		Context("when retrieving builds fails", func() {
			BeforeEach(func() {
				client.BuildsReturns(nil, concourse.Pagination{}, errors.New("hoho"))
			})

			It("should not send any builds", func() {
				Consistently(builds).ShouldNot(Receive())
			})

			It("should close the returned channel", func() {
				Eventually(builds).Should(BeClosed())
			})
		})

		Context("when retrieving builds succeeds", func() {
			var lastBuild int

			BeforeEach(func() {
				lastBuild = 41
				pg := concourse.Pagination{Next: &concourse.Page{Since: lastBuild}}
				client.BuildsReturnsOnCall(0, nil, pg, nil)
			})

			Context("and canceling the provided context", func() {
				It("should close the returned channel", func() {
					stop()
					Eventually(builds).Should(BeClosed())
				})
			})

			Context("but all builds are currently running", func() {
				BeforeEach(func() {
					builds := []atc.Build{
						atc.Build{Status: "pending"},
						atc.Build{Status: "started"},
					}
					client.BuildsReturnsOnCall(1, builds, concourse.Pagination{}, nil)
				})

				It("should retrieve builds", func() {
					Eventually(client.BuildsCallCount).Should(BeNumerically(">=", 2))
				})

				It("should not send any builds", func() {
					Consistently(builds).ShouldNot(Receive())
				})
			})

			Context("and there are finished builds", func() {
				var b1, b2 atc.Build
				BeforeEach(func() {
					b1, b2 = atc.Build{Status: "errored"}, atc.Build{Status: "succeeded"}
					builds := []atc.Build{
						b1,
						atc.Build{Status: "pending"},
						b2,
						atc.Build{Status: "started"},
					}
					pg := concourse.Pagination{Next: &concourse.Page{Since: lastBuild + 4}}
					client.BuildsReturnsOnCall(1, builds, pg, nil)
				})

				It("should retrieve builds", func() {
					Eventually(client.BuildsCallCount).Should(BeNumerically(">=", 2))
					argPage := client.BuildsArgsForCall(0)
					Ω(argPage.Limit).Should(Equal(1))

					argPage = client.BuildsArgsForCall(1)
					Ω(argPage.Until).Should(Equal(lastBuild))
					Ω(argPage.Limit).Should(Equal(100))
				})

				It("should send all finished builds in order", func() {
					var b atc.Build
					Eventually(builds).Should(Receive(&b))
					Ω(b).Should(Equal(b1))
					Eventually(builds).Should(Receive(&b))
					Ω(b).Should(Equal(b2))
				})
			})
		})
	})
})
