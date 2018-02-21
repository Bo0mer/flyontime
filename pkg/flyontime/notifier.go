package flyontime

type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

type Notification struct {
	Severity      Severity
	Title         string
	DashboardLink string
	JobOutput     string
}

type Notifier interface {
	Notify(n *Notification) error
}
