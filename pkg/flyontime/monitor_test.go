package flyontime_test

import (
	"context"
	"io"
	"time"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/Bo0mer/flyontime/pkg/flyontime"
	"github.com/Bo0mer/flyontime/pkg/flyontime/flyontimefakes"
	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	goconc "github.com/concourse/go-concourse/concourse"
)

var _ = Describe("JobMonitor", func() {

	var concourse *flyontimefakes.FakeConcourseClient
	var notifier *flyontimefakes.FakeNotifier

	var m *JobMonitor

	BeforeEach(func() {
		concourse = new(flyontimefakes.FakeConcourseClient)
		notifier = new(flyontimefakes.FakeNotifier)
		m = NewMonitor(concourse, notifier,
			WithPollInterval(50*time.Millisecond),
			WithLogger(lager.NewLogger("flyontime-test")))
	})

	It("should not be nil", func() {
		Ω(m).ShouldNot(BeNil())
	})

	Describe("Monitor", func() {
		var stop func()
		var lastBuild int

		BeforeEach(func() {
			lastBuild = 42
			concourse.BuildsReturns(nil, goconc.Pagination{Next: &goconc.Page{Since: lastBuild}}, nil)

			ctx, cancel := context.WithCancel(context.Background())
			stop = cancel
			go m.Monitor(ctx)
		})

		AfterEach(func() {
			stop()
		})

		It("should query for builds to get the last active build", func() {
			Eventually(concourse.BuildsCallCount).Should(Equal(1))
		})

		Context("when time passes", func() {
			BeforeEach(func() {

			})

			It("should check for new builds", func() {
				Eventually(concourse.BuildsCallCount).Should(Equal(2))
				argPage := concourse.BuildsArgsForCall(1)
				// All builds until the last it is aware of.
				Ω(argPage.Until).Should(Equal(lastBuild))
			})

			Context("and there are some new builds", func() {
				Context("and one of them has failed", func() {
					BeforeEach(func() {
						builds := []atc.Build{
							atc.Build{ID: 43, TeamName: "main", PipelineName: "pip", JobName: "cactus", Status: statusSucceeded},
							atc.Build{ID: 44, TeamName: "main", PipelineName: "pip", JobName: "cactus", Status: statusFailed},
						}
						pg := goconc.Pagination{Next: &goconc.Page{Since: lastBuild + 2}}
						concourse.BuildsReturns(builds, pg, nil)

						events := new(flyontimefakes.FakeConcourseEvents)
						events.NextEventReturnsOnCall(0, event.InitializeTask{
							TaskConfig: event.TaskConfig{
								Run: event.TaskRunConfig{
									Path: "echo",
									Args: []string{"hello"},
								},
							},
						}, nil)
						events.NextEventReturnsOnCall(1, event.StartTask{}, nil)
						events.NextEventReturnsOnCall(2, event.Log{Payload: "hello"}, nil)
						events.NextEventReturnsOnCall(3, event.FinishTask{ExitStatus: 1}, nil)
						events.NextEventReturnsOnCall(4, nil, io.EOF)
						concourse.BuildEventsReturns(events, nil)
					})

					It("should retrieve all build events for the failed build", func() {
						Eventually(concourse.BuildEventsCallCount).Should(Equal(1))
						argID := concourse.BuildEventsArgsForCall(0)
						Ω(argID).Should(Equal("44"))
					})

					It("should send a notification including the failed job output", func() {
						Eventually(notifier.NotifyCallCount).Should(Equal(1))
						argNotification := notifier.NotifyArgsForCall(0)
						Ω(argNotification.JobOutput).Should(Equal("command: echo hello\nhello\nexit code 1"))
					})
				})
			})
		})
	})

})

const (
	statusFailed    = string(atc.StatusFailed)
	statusSucceeded = string(atc.StatusSucceeded)
)
