package flyontime

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/concourse/go-concourse/concourse"
)

type Builder interface {
	Builds() <-chan atc.Build
}

type Monitor struct {
	pilot *Pilot
	log   lager.Logger

	cancel context.CancelFunc // signals to build producer to stop.
	builds <-chan atc.Build

	commands <-chan *Command
	stop     chan struct{}

	history         map[jobKey]*jobHistory
	notifiers       map[jobStatus]func(atc.Build, *jobHistory) error
	manuallyStarted map[int]func(b atc.Build)
}

func NewMonitor(pilot *Pilot, n Notifier, c Commander) *Monitor {
	log := lager.NewLogger("flyontime")
	log.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	ctx, cancel := context.WithCancel(context.Background())
	m := &Monitor{
		pilot: pilot,
		log:   log,

		cancel: cancel,
		builds: pilot.FinishedBuilds(ctx),

		commands: c.Commands(),
		stop:     make(chan struct{}),

		history:         make(map[jobKey]*jobHistory),
		notifiers:       defaultNotifiers(n, pilot.Client),
		manuallyStarted: make(map[int]func(atc.Build)),
	}

	return m
}

func (m *Monitor) Start() {
	m.run()
}

func (m *Monitor) Stop() {
	m.cancel()
	close(m.stop)
}

func (m *Monitor) run() {
	for {
		select {
		case b := <-m.builds:
			m.handleBuild(b)
		case c := <-m.commands:
			m.handleCommand(c)
		case <-m.stop:
			return
		}
	}
}

func (m *Monitor) handleCommand(c *Command) {
	switch c.Name {
	case "rerun", "try again", "retry":
		m.commandRerun(c)
	case "pause", "stop":
		m.commandPause(c)
	case "unpause", "play":
		m.commandPlay(c)
	default:
		c.Responses <- fmt.Sprintf("Unknown command: %q", c.Name)
	}
}

func (m *Monitor) commandRerun(c *Command) {
	j := c.Job
	b, err := m.pilot.CreateJobBuild(j.Pipeline, j.Name)
	if err != nil {
		c.Responses <- fmt.Sprintf("Running %s failed: %v", c.Job.Name, err)
		close(c.Responses)
		return
	}
	c.Responses <- fmt.Sprintf("Rerunning %s...", c.Job.Name)
	m.manuallyStarted[b.ID] = func(b atc.Build) {
		c.Responses <- fmt.Sprintf("Job %s.", b.Status)
		close(c.Responses)
	}
}

func (m *Monitor) commandPause(c *Command) {
	defer close(c.Responses)

	ok, err := m.pilot.PausePipeline(c.Job.Pipeline)
	if err != nil {
		c.Responses <- fmt.Sprintf("Pausing pipeline %s failed: %v", c.Job.Pipeline, err)
	}
	if ok {
		c.Responses <- fmt.Sprintf("Pipeline %s is now paused.", c.Job.Pipeline)
	} else {
		c.Responses <- fmt.Sprintf("Pipeline %s is already paused.", c.Job.Pipeline)
	}
}

func (m *Monitor) commandPlay(c *Command) {
	defer close(c.Responses)

	ok, err := m.pilot.UnpausePipeline(c.Job.Pipeline)
	if err != nil {
		c.Responses <- fmt.Sprintf("Unpausing pipeline %s failed: %v", c.Job.Pipeline, err)
	}
	if ok {
		c.Responses <- fmt.Sprintf("Pipeline %s is now unapused.", c.Job.Pipeline)
	} else {
		c.Responses <- fmt.Sprintf("Pipeline %s is already unpaused.", c.Job.Pipeline)
	}
}

func (m *Monitor) handleBuild(b atc.Build) {
	if b.OneOff() {
		// One off, no need to send notifications.
		return
	}

	h, ok := m.history[jobKey{b.TeamName, b.PipelineName, b.JobName}]
	if !ok {
		h = &jobHistory{}
	}
	defer m.updateHistory(b, h)

	if respond, ok := m.isManuallyStarted(b); ok {
		// Respond with the build status if it is manually started.
		respond(b)
		delete(m.manuallyStarted, b.ID)
		return
	}

	// Send notification if necessary.
	if m.shouldNotify(b, h) {
		m.notify(b, h)
	}
}

func (m *Monitor) isManuallyStarted(build atc.Build) (respond func(atc.Build), ok bool) {
	respond, ok = m.manuallyStarted[build.ID]
	return
}

func (m *Monitor) shouldNotify(build atc.Build, h *jobHistory) bool {
	_, ok := m.notifiers[jobStatus{h.LastStatus, build.Status}]
	return ok
}

func (m *Monitor) notify(build atc.Build, h *jobHistory) {
	f, ok := m.notifiers[jobStatus{h.LastStatus, build.Status}]
	if !ok {
		return
	}
	if err := f(build, h); err != nil {
		m.log.Session("notify").Error("fail", err)
	}
}

func (m *Monitor) updateHistory(b atc.Build, h *jobHistory) {
	if b.Status == statusSucceeded {
		h.ConsecutiveFailures = 0
	}
	if b.Status == statusFailed {
		h.ConsecutiveFailures++
	}
	h.LastStatus = b.Status
	m.history[jobKey{b.TeamName, b.PipelineName, b.JobName}] = h
}

func min(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func buildOutput(c concourse.Client, b atc.Build) (string, error) {
	events, err := c.BuildEvents(strconv.Itoa(b.ID))
	if err != nil {
		return "", err
	}
	if events == nil {
		return "", nil
	}
	defer events.Close()

	var sb strings.Builder
	var buildConfig event.TaskConfig
	for {
		ev, err := events.NextEvent()
		if err != nil {
			if err == io.EOF {
				return sb.String(), nil
			} else {
				return "", fmt.Errorf("flyontime: failed to parse event: %v", err)
			}
		}

		switch e := ev.(type) {
		case event.Log:
			sb.WriteString(e.Payload)

		case event.InitializeTask:
			buildConfig = e.TaskConfig

		case event.StartTask:
			argv := strings.Join(append([]string{buildConfig.Run.Path}, buildConfig.Run.Args...), " ")
			fmt.Fprintf(&sb, "command: %s\n", argv)

		case event.Error:
			fmt.Fprintf(&sb, "%s\n", e.Message)

		case event.FinishTask:
			fmt.Fprintf(&sb, "\nexit code %d", e.ExitStatus)
		}
	}
}

const (
	statusFailed    = string(atc.StatusFailed)
	statusSucceeded = string(atc.StatusSucceeded)
	statusErrored   = string(atc.StatusErrored)
	statusAborted   = string(atc.StatusAborted)
)

type jobHistory struct {
	LastStatus          string
	ConsecutiveFailures int
}

type jobKey struct {
	Team     string
	Pipeline string
	Job      string
}

type jobStatus struct {
	Old string
	New string
}

func defaultNotifiers(n Notifier, concourse concourse.Client) map[jobStatus]func(atc.Build, *jobHistory) error {
	dashboardLink := func(b atc.Build) string {
		return fmt.Sprintf("%s/teams/%s/pipelines/%s/jobs/%s/builds/%s", concourse.URL(), b.TeamName, b.PipelineName, b.JobName, b.Name)
	}

	// errored is used for all states that transition into errored build.
	errored := func(b atc.Build, h *jobHistory) error {
		return n.Notify(&Notification{
			Severity:      SeverityError,
			Title:         fmt.Sprintf("Job %s from %s has errored.", b.JobName, b.PipelineName),
			Job:           jobFromATCBuild(b),
			DashboardLink: dashboardLink(b),
		})
	}

	return map[jobStatus]func(atc.Build, *jobHistory) error{
		{statusSucceeded, statusFailed}: func(b atc.Build, h *jobHistory) error {
			output, _ := buildOutput(concourse, b)
			return n.Notify(&Notification{
				Severity:      SeverityError,
				Title:         fmt.Sprintf("Job %s from %s has failed.", b.JobName, b.PipelineName),
				DashboardLink: dashboardLink(b),
				Job:           jobFromATCBuild(b),
				JobOutput:     output,
			})
		},
		{statusFailed, statusFailed}: func(b atc.Build, h *jobHistory) error {
			output, err := buildOutput(concourse, b)
			if err != nil {
				output = ""
			}
			return n.Notify(&Notification{
				Severity:      SeverityError,
				Title:         fmt.Sprintf("Job %s from %s is still failing (%d times in a row).", b.JobName, b.PipelineName, h.ConsecutiveFailures),
				DashboardLink: dashboardLink(b),
				Job:           jobFromATCBuild(b),
				JobOutput:     output,
			})
		},
		{statusFailed, statusSucceeded}: func(b atc.Build, h *jobHistory) error {
			return n.Notify(&Notification{
				Severity:      SeverityInfo,
				Title:         fmt.Sprintf("Job %s from %s has recovered after %d failure(s).", b.JobName, b.PipelineName, h.ConsecutiveFailures),
				Job:           jobFromATCBuild(b),
				DashboardLink: dashboardLink(b),
			})
		},
		{"", statusErrored}:              errored,
		{statusSucceeded, statusErrored}: errored,
		{statusFailed, statusErrored}:    errored,
		{statusErrored, statusErrored}:   errored,
	}
}
