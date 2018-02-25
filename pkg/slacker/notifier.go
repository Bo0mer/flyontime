package slacker

import (
	"fmt"
	"sync"

	"github.com/Bo0mer/flyontime/pkg/flyontime"
	"github.com/nlopes/slack"
)

type Notifier struct {
	Token     string
	ChannelID string

	initOnce sync.Once
	slack    *slack.Client
}

func (s *Notifier) init() {
	s.initOnce.Do(func() {
		s.slack = slack.New(s.Token)
	})
}

func (s *Notifier) Notify(n *flyontime.Notification) error {
	s.init()
	p := slack.PostMessageParameters{
		Attachments: []slack.Attachment{
			slack.Attachment{
				Color:      colorFor(n.Severity),
				AuthorName: "Concourse",
				AuthorIcon: "",
				Title:      n.Title,
				TitleLink:  n.DashboardLink,
				Text:       formatCode(n.JobOutput),
				MarkdownIn: []string{"text"},
			},
		},
	}
	_, _, err := s.slack.PostMessage(s.ChannelID, "", p)
	return err
}

func colorFor(severity flyontime.Severity) string {
	if severity == flyontime.SeverityInfo {
		return "good"
	}
	return "danger"
}

func formatCode(code string) string {
	if code == "" {
		return ""
	}
	return fmt.Sprintf("```%s```", code)
}
