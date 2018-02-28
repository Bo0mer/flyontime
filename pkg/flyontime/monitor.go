package flyontime

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/concourse/go-concourse/concourse"
)

//go:generate counterfeiter . Pilot

type Pilot interface {
	URL() string
	PausePipeline(pipeline string) (bool, error)
	UnpausePipeline(pipeline string) (bool, error)
	PauseJob(pipeline, job string) (bool, error)
	UnpauseJob(pipeline, job string) (bool, error)
	CreateJobBuild(pipeline, job string) (atc.Build, error)
	BuildEvents(job string) (concourse.Events, error)
	FinishedBuilds(ctx context.Context) <-chan atc.Build
}

type Monitor struct {
	pilot Pilot
	log   lager.Logger

	cancel context.CancelFunc // signals to build producer to stop.
	builds <-chan atc.Build

	commands <-chan *Command
	stop     chan struct{}

	history         map[jobKey]*jobHistory
	notifiers       map[jobStatus]notifyFunc
	manuallyStarted map[int]func(b atc.Build)

	mu    sync.Mutex
	muted map[jobKey]time.Time
}

type notifyFunc func(context.Context, atc.Build, *jobHistory) error

func NewMonitor(pilot Pilot, n Notifier, c Commander, logger lager.Logger) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Monitor{
		pilot: pilot,
		log:   logger,

		cancel: cancel,
		builds: pilot.FinishedBuilds(ctx),

		commands: c.Commands(),
		stop:     make(chan struct{}),

		history:         make(map[jobKey]*jobHistory),
		notifiers:       defaultNotifiers(n, pilot),
		manuallyStarted: make(map[int]func(atc.Build)),
		muted:           make(map[jobKey]time.Time),
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
			m.handleBuild(m.log.Session("handle-build"), b)
		case c := <-m.commands:
			m.handleCommand(m.log.Session("handle-command"), c)
		case <-m.stop:
			return
		}
	}
}

func (m *Monitor) handleCommand(logger lager.Logger, c *Command) {
	logger.Info(c.Name, lager.Data{"args": c.Args})
	switch c.Name {
	case "rerun", "try again", "retry":
		m.commandRerun(c)
	case "pause", "stop":
		m.commandPause(c)
	case "unpause", "play":
		m.commandPlay(c)
	case "mute", "silence":
		m.commandMute(c)
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

	if len(c.Args) > 0 && c.Args[0] == "pipeline" {
		ok, err := m.pilot.PausePipeline(c.Job.Pipeline)
		if err != nil {
			c.Responses <- fmt.Sprintf("Pausing pipeline %s failed: %v", c.Job.Pipeline, err)
		}
		if ok {
			c.Responses <- fmt.Sprintf("Pipeline %s is now paused.", c.Job.Pipeline)
		} else {
			c.Responses <- fmt.Sprintf("Pipeline %s is already paused.", c.Job.Pipeline)
		}
		return
	}

	ok, err := m.pilot.PauseJob(c.Job.Pipeline, c.Job.Name)
	if err != nil {
		c.Responses <- fmt.Sprintf("Pausing job %s failed: %v", c.Job.Name, err)
	}
	if ok {
		c.Responses <- fmt.Sprintf("Job %s is now paused.", c.Job.Name)
	} else {
		c.Responses <- fmt.Sprintf("Job %s is already paused.", c.Job.Name)
	}
	return
}

func (m *Monitor) commandPlay(c *Command) {
	defer close(c.Responses)

	if len(c.Args) > 0 && c.Args[0] == "pipeline" {
		ok, err := m.pilot.UnpausePipeline(c.Job.Pipeline)
		if err != nil {
			c.Responses <- fmt.Sprintf("Unpausing pipeline %s failed: %v", c.Job.Pipeline, err)
		}
		if ok {
			c.Responses <- fmt.Sprintf("Pipeline %s is now unpaused.", c.Job.Pipeline)
		} else {
			c.Responses <- fmt.Sprintf("Pipeline %s is already unpaused.", c.Job.Pipeline)
		}
		return
	}

	ok, err := m.pilot.UnpauseJob(c.Job.Pipeline, c.Job.Name)
	if err != nil {
		c.Responses <- fmt.Sprintf("Unpausing job %s failed: %v", c.Job.Name, err)
	}
	if ok {
		c.Responses <- fmt.Sprintf("Job %s is now unpaused.", c.Job.Name)
	} else {
		c.Responses <- fmt.Sprintf("Job %s is already unpaused.", c.Job.Name)
	}
	return
}

func (m *Monitor) commandMute(c *Command) {
	defer close(c.Responses)

	if len(c.Args) == 0 {
		c.Args = []string{"30m"} // default mute duration
	}
	d, err := time.ParseDuration(c.Args[0])
	if err != nil {
		c.Responses <- fmt.Sprintf("Invalid duration %q", c.Args[0])
		return
	}

	j := c.Job
	until := time.Now().Add(d)
	m.mu.Lock()
	m.muted[jobKey{j.Team, j.Pipeline, j.Name}] = until
	m.mu.Unlock()
	c.Responses <- fmt.Sprintf("Muted notifications for %s until %s", j.Name, until.Format(time.Kitchen))
}

func (m *Monitor) handleBuild(logger lager.Logger, b atc.Build) {
	if b.OneOff() {
		logger.Info("skip-one-off")
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
		logger.Info("respond-to-manually-started")
		respond(b)
		delete(m.manuallyStarted, b.ID)
		return
	}

	// Send notification if necessary.
	if m.shouldNotify(logger.Session("should-notify"), b, h) {
		m.notify(
			logger.Session("notify", lager.Data{"build": b.ID}),
			b, h)
	}
}

func (m *Monitor) isManuallyStarted(build atc.Build) (respond func(atc.Build), ok bool) {
	respond, ok = m.manuallyStarted[build.ID]
	return
}

func (m *Monitor) shouldNotify(logger lager.Logger, b atc.Build, h *jobHistory) bool {
	_, ok := m.notifiers[jobStatus{h.LastStatus, b.Status}]
	if !ok {
		logger.Debug("no-notifier")
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	key := jobKey{b.TeamName, b.PipelineName, b.JobName}
	until, muted := m.muted[key]
	if !muted {
		return ok
	}

	if time.Now().Before(until) {
		logger.Debug("notifications-muted")
		return false
	}
	delete(m.muted, key)
	return ok
}

func (m *Monitor) notify(logger lager.Logger, build atc.Build, h *jobHistory) {
	f, ok := m.notifiers[jobStatus{h.LastStatus, build.Status}]
	if !ok {
		return
	}
	ctx := lagerctx.NewContext(context.Background(), logger)
	if err := f(ctx, build, h); err != nil {
		logger.Error("fail", err)
		return
	}
	logger.Info("done")
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

func buildOutput(ctx context.Context, c Pilot, b atc.Build) string {
	logger := lagerctx.WithSession(ctx, "get-build-output")
	events, err := c.BuildEvents(strconv.Itoa(b.ID))
	if err != nil {
		logger.Error("get-build-events.fail", err)
		return ""
	}
	if events == nil {
		logger.Info("no-build-events")
		return ""
	}
	defer events.Close()

	var sb strings.Builder
	var buildConfig event.TaskConfig
	for {
		ev, err := events.NextEvent()
		if err != nil {
			if err == io.EOF {
				return sb.String()
			} else {
				logger.Error("parse-event-fail-will-skip", err)
				continue
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

func defaultNotifiers(n Notifier, concourse Pilot) map[jobStatus]notifyFunc {
	dashboardLink := func(b atc.Build) string {
		return fmt.Sprintf("%s/teams/%s/pipelines/%s/jobs/%s/builds/%s", concourse.URL(), b.TeamName, b.PipelineName, b.JobName, b.Name)
	}

	// errored is used for all states that transition into errored build.
	errored := func(ctx context.Context, b atc.Build, h *jobHistory) error {
		return n.Notify(ctx, &Notification{
			Severity:      SeverityError,
			Title:         fmt.Sprintf("Job %s from %s has errored.", b.JobName, b.PipelineName),
			Job:           jobFromATCBuild(b),
			DashboardLink: dashboardLink(b),
		})
	}

	// errored is used for all states that transition into aborted build.
	aborted := func(ctx context.Context, b atc.Build, h *jobHistory) error {
		return n.Notify(ctx, &Notification{
			Severity:      SeverityError,
			Title:         fmt.Sprintf("Job %s from %s has been aborted.", b.JobName, b.PipelineName),
			Job:           jobFromATCBuild(b),
			DashboardLink: dashboardLink(b),
		})
	}

	failed := func(ctx context.Context, b atc.Build, h *jobHistory) error {
		output := buildOutput(ctx, concourse, b)
		return n.Notify(ctx, &Notification{
			Severity:      SeverityError,
			Title:         fmt.Sprintf("Job %s from %s has failed.", b.JobName, b.PipelineName),
			DashboardLink: dashboardLink(b),
			Job:           jobFromATCBuild(b),
			JobOutput:     output,
		})
	}

	return map[jobStatus]notifyFunc{
		{"", statusFailed}:              failed,
		{statusSucceeded, statusFailed}: failed,
		{statusFailed, statusFailed}: func(ctx context.Context, b atc.Build, h *jobHistory) error {
			output := buildOutput(ctx, concourse, b)
			return n.Notify(ctx, &Notification{
				Severity:      SeverityError,
				Title:         fmt.Sprintf("Job %s from %s is still failing (%d times in a row).", b.JobName, b.PipelineName, h.ConsecutiveFailures),
				DashboardLink: dashboardLink(b),
				Job:           jobFromATCBuild(b),
				JobOutput:     output,
			})
		},
		{statusFailed, statusSucceeded}: func(ctx context.Context, b atc.Build, h *jobHistory) error {
			return n.Notify(ctx, &Notification{
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
		{"", statusAborted}:              aborted,
		{statusSucceeded, statusAborted}: aborted,
		{statusFailed, statusAborted}:    aborted,
		{statusErrored, statusAborted}:   aborted,
	}
}

//go:generate counterfeiter . concourseEvents

type concourseEvents interface {
	concourse.Events
}
