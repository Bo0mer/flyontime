package flyontime

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/concourse/go-concourse/concourse"
)

type concourseClient interface {
	concourse.Client
}

type concourseEvents interface {
	concourse.Events
}

//go:generate counterfeiter . concourseClient
//go:generate counterfeiter . concourseEvents
//go:generate counterfeiter . Notifier

type JobMonitor struct {
	concourse concourse.Client

	pollInterval time.Duration
	history      map[jobKey]*jobHistory
	notifiers    map[jobStatus]func(atc.Build, *jobHistory) error

	log lager.Logger
}

type Option func(*JobMonitor)

func WithPollInterval(d time.Duration) Option {
	return func(m *JobMonitor) {
		m.pollInterval = d
	}
}

func WithLogger(log lager.Logger) Option {
	return func(m *JobMonitor) {
		m.log = log
	}
}

func NewMonitor(concourse concourse.Client, n Notifier, opts ...Option) *JobMonitor {
	log := lager.NewLogger("flyontime")
	log.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	dashboardLink := func(buildURL string) string {
		return fmt.Sprintf("%s%s", concourse.URL(), buildURL)
	}
	m := &JobMonitor{
		concourse:    concourse,
		log:          log,
		pollInterval: 4 * time.Second,
		history:      make(map[jobKey]*jobHistory),
		notifiers: map[jobStatus]func(atc.Build, *jobHistory) error{
			{statusSucceeded, statusFailed}: func(b atc.Build, h *jobHistory) error {
				output, _ := buildOutput(concourse, b)
				return n.Notify(&Notification{
					Severity:      SeverityError,
					Title:         fmt.Sprintf("Job %s from %s has failed.", b.JobName, b.PipelineName),
					DashboardLink: dashboardLink(b.URL),
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
					DashboardLink: dashboardLink(b.URL),
					JobOutput:     output,
				})
			},
			{statusFailed, statusSucceeded}: func(b atc.Build, h *jobHistory) error {
				return n.Notify(&Notification{
					Severity:      SeverityInfo,
					Title:         fmt.Sprintf("Job %s from %s has recovered after %d failure(s).", b.JobName, b.PipelineName, h.ConsecutiveFailures),
					DashboardLink: dashboardLink(b.URL),
				})
			},
		},
	}

	for _, op := range opts {
		op(m)
	}
	return m
}

func (n *JobMonitor) Monitor(ctx context.Context) {
	builds := n.jobs(ctx)

	for b := range builds {
		h, ok := n.history[jobKey{b.TeamName, b.PipelineName, b.JobName}]
		if !ok {
			h = &jobHistory{}
		}

		if n.shouldNotify(b, h) {
			n.notify(b, h)
		}
		n.updateHistory(b, h)
	}
}

func (n *JobMonitor) shouldNotify(build atc.Build, h *jobHistory) bool {
	n.log.Debug("should-notify", lager.Data{"build": build, "history": h})
	_, ok := n.notifiers[jobStatus{h.LastStatus, build.Status}]
	return ok
}

func (n *JobMonitor) notify(build atc.Build, h *jobHistory) {
	f, ok := n.notifiers[jobStatus{h.LastStatus, build.Status}]
	if !ok {
		return
	}
	if err := f(build, h); err != nil {
		n.log.Session("notify").Error("fail", err)
	}
}

func (n *JobMonitor) updateHistory(b atc.Build, h *jobHistory) {
	if b.Status == statusSucceeded {
		h.ConsecutiveFailures = 0
	}
	if b.Status == statusFailed {
		h.ConsecutiveFailures++
	}
	h.LastStatus = b.Status
	n.history[jobKey{b.TeamName, b.PipelineName, b.JobName}] = h
}

func (n *JobMonitor) jobs(ctx context.Context) <-chan atc.Build {
	log := n.log.Session("get-jobs")
	c := make(chan atc.Build)
	go func() {
		t := time.NewTicker(n.pollInterval)
		defer func() {
			t.Stop()
			close(c)
		}()
		_, p, err := n.concourse.Builds(concourse.Page{Limit: 1})
		if err != nil {
			log.Error("fail", err)
			return
		}
		lastSeen := p.Next.Since
		var builds []atc.Build
		for {
			select {
			case <-t.C:
				log := log.Session("recheck")
				log.Debug("start")
				builds, p, err = n.concourse.Builds(concourse.Page{Until: lastSeen, Limit: 100})
				if err != nil {
					log.Error("fail", err)
					continue
				}
				if p.Next == nil {
					// No new builds.
					continue
				}
				// If there are more than 100 new builds.
				if last := p.Next.Since; last-lastSeen > 100 {
					lastSeen = lastSeen + 100
				} else {
					lastSeen = p.Next.Since
				}

				for _, b := range builds {
					if b.IsRunning() {
						// In order to resend the build once it has finished.
						lastSeen = min(b.ID, lastSeen) - 1
						continue
					}
					c <- b
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return c
}

func (m *JobMonitor) lastBuild() (int, error) {
	_, p, err := m.concourse.Builds(concourse.Page{Limit: 1})
	if err != nil {
		return 0, err
	}
	return p.Next.Since, nil
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
