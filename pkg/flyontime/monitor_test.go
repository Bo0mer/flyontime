package flyontime_test

import (
	"errors"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/Bo0mer/flyontime/pkg/flyontime"
	"github.com/Bo0mer/flyontime/pkg/flyontime/flyontimefakes"
)

var _ = Describe("Monitor", func() {

	var commander *flyontimefakes.FakeCommander
	var notifier *flyontimefakes.FakeNotifier
	var pilot *flyontimefakes.FakePilot

	var monitor *Monitor

	BeforeEach(func() {
		commander = new(flyontimefakes.FakeCommander)
		notifier = new(flyontimefakes.FakeNotifier)
		pilot = new(flyontimefakes.FakePilot)
	})

	AfterEach(func() {
		monitor.Stop()
	})

	JustBeforeEach(func() {
		logger := lager.NewLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
		monitor = NewMonitor(pilot, notifier, commander, logger)
		go monitor.Start()
	})

	Context("when a commands comes in", func() {
		var commands chan *Command
		var c *Command
		var responses chan string

		var builds chan atc.Build

		BeforeEach(func() {
			responses = make(chan string, 1)
			c = &Command{Responses: responses}

			commands = make(chan *Command, 1)
			commander.CommandsReturns(commands)

			builds = make(chan atc.Build, 2)
			pilot.FinishedBuildsReturns(builds)
		})

		Context("but it is unknown", func() {
			BeforeEach(func() {
				c.Name = "oho-boho"
				commands <- c
			})

			It("should reply with 'unknown command'", func() {
				var resp string
				Eventually(responses).Should(Receive(&resp))
				Ω(resp).Should(ContainSubstring("Unknown command"))
			})
		})

		Context("and it is rerun", func() {
			BeforeEach(func() {
				c.Name = "rerun"
				c.Job = &Job{
					Team:     "t1",
					Pipeline: "p1",
					Name:     "j1",
				}
				commands <- c
			})

			It("should rerun the job", func() {
				Eventually(pilot.CreateJobBuildCallCount).Should(Equal(1))
				argPipeline, argJob := pilot.CreateJobBuildArgsForCall(0)
				Ω(argPipeline).Should(Equal("p1"))
				Ω(argJob).Should(Equal("j1"))
			})

			It("should say that it is rerunning the job", func() {
				var resp string
				Eventually(responses).Should(Receive(&resp))
				Ω(resp).Should(ContainSubstring("Rerunning j1"))
			})

			Context("and the rerunned job finishes", func() {
				BeforeEach(func() {
					b := atc.Build{ID: 42}
					pilot.CreateJobBuildStub = func(_, _ string) (atc.Build, error) {
						// Simulate that job finishes shortly.
						time.AfterFunc(5*time.Millisecond*durationScaleFactor, func() {
							builds <- atc.Build{
								ID:      b.ID,
								Status:  "succeeded",
								JobName: "j1",
							}
						})
						return b, nil
					}
				})

				It("replies with the build result", func() {
					var resp string
					Eventually(responses).Should(Receive(&resp))
					// NOTE(borshukov): The first receive will result in "Rerunning j1"
					Eventually(responses).Should(Receive(&resp))
					Ω(resp).Should(ContainSubstring("Job succeeded"))
				})
			})
		})

		Context("and it is mute", func() {
			BeforeEach(func() {
				c.Name = "mute"
				c.Args = []string{fmt.Sprintf("%dms", 20*int(durationScaleFactor))}
				c.Job = &Job{
					Team:     "t1",
					Pipeline: "p1",
					Name:     "j1",
				}
				commands <- c
			})

			It("should say that it is muting notifications for the job", func() {
				var resp string
				Eventually(responses).Should(Receive(&resp))
				Ω(resp).Should(ContainSubstring("Muted notifications for j1 until"))
			})

			Context("and the job should trigger notification again", func() {
				BeforeEach(func() {
					time.AfterFunc(5*time.Millisecond*durationScaleFactor, func() {
						// Simulate delay.
						builds <- atc.Build{
							ID:           42,
							Status:       "errored",
							TeamName:     "t1",
							PipelineName: "p1",
							JobName:      "j1",
						}
					})
				})

				It("does not send any notification", func() {
					Consistently(notifier.NotifyCallCount).Should(Equal(0))
				})
			})

			Context("and the mute duration passes", func() {
				BeforeEach(func() {
					time.AfterFunc(40*time.Millisecond*durationScaleFactor, func() {
						// Simulate delay.
						builds <- atc.Build{
							ID:           42,
							Status:       "errored",
							TeamName:     "t1",
							PipelineName: "p1",
							JobName:      "j1",
						}
					})
				})

				It("starts sending notifications again", func() {
					Eventually(notifier.NotifyCallCount).Should(Equal(1))
				})
			})
		})

		Context("and it is pause", func() {
			BeforeEach(func() {
				c.Name = "pause"
				c.Job = &Job{
					Team:     "t1",
					Pipeline: "p1",
					Name:     "j1",
				}
				commands <- c
			})

			Context("and pausing the pipeline succeeds", func() {

				It("should pause the pipeline that the job is part of", func() {
					Eventually(pilot.PausePipelineCallCount).Should(Equal(1))
					argPipeline := pilot.PausePipelineArgsForCall(0)
					Ω(argPipeline).Should(Equal("p1"))
				})

				Context("but it is already pasued", func() {
					BeforeEach(func() {
						pilot.PausePipelineReturns(false, nil)
					})

					It("still replies with success", func() {
						var resp string
						Eventually(responses).Should(Receive(&resp))
						Ω(resp).Should(ContainSubstring("Pipeline p1 is already paused"))
					})
				})

				Context("and it is not already paused", func() {
					BeforeEach(func() {
						pilot.PausePipelineReturns(true, nil)
					})

					It("replies with success", func() {
						var resp string
						Eventually(responses).Should(Receive(&resp))
						Ω(resp).Should(ContainSubstring("Pipeline p1 is now paused"))
					})
				})
			})

			Context("and pausing the pipeline fails", func() {
				BeforeEach(func() {
					pilot.PausePipelineReturns(false, errors.New("kuku"))
				})

				It("replies with failure", func() {
					var resp string
					Eventually(responses).Should(Receive(&resp))
					Ω(resp).Should(ContainSubstring("Pausing pipeline p1 failed: kuku"))
				})
			})
		})

		Context("and it is play", func() {
			BeforeEach(func() {
				c.Name = "play"
				c.Job = &Job{
					Team:     "t1",
					Pipeline: "p1",
					Name:     "j1",
				}
				commands <- c
			})

			Context("and unapusing the pipeline succeeds", func() {

				It("should unpause the pipeline that the job is part of", func() {
					Eventually(pilot.UnpausePipelineCallCount).Should(Equal(1))
					argPipeline := pilot.UnpausePipelineArgsForCall(0)
					Ω(argPipeline).Should(Equal("p1"))
				})

				Context("but it is not pasued", func() {
					BeforeEach(func() {
						pilot.UnpausePipelineReturns(false, nil)
					})

					It("still replies with success", func() {
						var resp string
						Eventually(responses).Should(Receive(&resp))
						Ω(resp).Should(ContainSubstring("Pipeline p1 is already unpaused"))
					})
				})

				Context("and it is not unpaused", func() {
					BeforeEach(func() {
						pilot.UnpausePipelineReturns(true, nil)
					})

					It("replies with success", func() {
						var resp string
						Eventually(responses).Should(Receive(&resp))
						Ω(resp).Should(ContainSubstring("Pipeline p1 is now unpaused"))
					})
				})
			})

			Context("and unpausing the pipeline fails", func() {
				BeforeEach(func() {
					pilot.UnpausePipelineReturns(false, errors.New("kuku"))
				})

				It("replies with failure", func() {
					var resp string
					Eventually(responses).Should(Receive(&resp))
					Ω(resp).Should(ContainSubstring("Unpausing pipeline p1 failed: kuku"))
				})
			})
		})
	})

	Context("when a build state changes", func() {
		var builds chan atc.Build
		BeforeEach(func() {
			builds = make(chan atc.Build, 2)
			pilot.FinishedBuildsReturns(builds)
		})

		DescribeTable("Notification Transitions",
			func(b1, b2 atc.Build, notifications int) {
				builds <- b1
				builds <- b2

				if notifications > 0 {
					Eventually(notifier.NotifyCallCount).Should(Equal(notifications))
				} else {
					Consistently(notifier.NotifyCallCount).Should(Equal(notifications))
				}
			},
			Entry("no status -> succeeded", build(""), build("succeeded"), 0),
			Entry("succeeded -> succeeded", build("succeeded"), build("succeeded"), 0),

			Entry("succeeded -> aborted", build("succeeded"), build("aborted"), 1),
			Entry("succeeded -> errored", build("succeeded"), build("errored"), 1),
			Entry("succeeded -> failed", build("succeeded"), build("failed"), 1),
			Entry("no status -> failed", build(""), build("failed"), 1),
			Entry("no status -> aborted", build(""), build("aborted"), 1),
			Entry("no status -> errored", build(""), build("errored"), 1),
			Entry("aborted -> succeeded", build("aborted"), build("succeeded"), 1),
			Entry("errored -> succeeded", build("errored"), build("succeeded"), 1),

			Entry("failed -> failed", build("failed"), build("failed"), 2),
			Entry("failed -> succeeded", build("failed"), build("succeeded"), 2),
			Entry("failed -> aborted", build("failed"), build("aborted"), 2),
			Entry("errored -> aborted", build("errored"), build("aborted"), 2),
			Entry("aborted -> errored", build("errored"), build("aborted"), 2),
			Entry("aborted -> failed", build("errored"), build("aborted"), 2),
		)
	})

	Context("when a build fails", func() {

		var builds chan atc.Build
		BeforeEach(func() {
			builds = make(chan atc.Build, 1)
			pilot.FinishedBuildsReturns(builds)
		})

		Context("and it is a one-off build", func() {
			BeforeEach(func() {
				builds <- atc.Build{JobName: "", Status: "failed"}
			})

			It("should not send a notification", func() {
				Consistently(notifier.NotifyCallCount).Should(Equal(0))
			})
		})

		Context("and it is not a one-off", func() {
			BeforeEach(func() {
				builds <- atc.Build{JobName: "job", Status: "failed"}
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
				pilot.BuildEventsReturns(events, nil)
			})

			It("should send a notification with the build output", func() {
				Eventually(notifier.NotifyCallCount).Should(Equal(1))
				_, argNotification := notifier.NotifyArgsForCall(0)
				Ω(argNotification.JobOutput).Should(Equal("command: echo hello\nhello\nexit code 1"))
			})
		})
	})
})

func build(status string) atc.Build {
	return atc.Build{
		JobName: "job",
		Status:  status,
	}
}
