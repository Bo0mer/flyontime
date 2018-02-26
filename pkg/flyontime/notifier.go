package flyontime

import "github.com/concourse/atc"

type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

type Job struct {
	Name     string
	Pipeline string
	Team     string
}

func jobFromATCBuild(b atc.Build) Job {
	return Job{
		Name:     b.JobName,
		Pipeline: b.PipelineName,
		Team:     b.TeamName,
	}
}

type Notification struct {
	Severity      Severity
	Title         string
	DashboardLink string
	Job           Job
	JobOutput     string
}

type Notifier interface {
	Notify(n *Notification) error
}
