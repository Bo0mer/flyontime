package slacker

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Bo0mer/flyontime/pkg/flyontime"
	"github.com/nlopes/slack"
	uuid "github.com/satori/go.uuid"
)

type Notifier struct {
	Token     string
	ChannelID string

	initOnce sync.Once
	slack    *slack.Client

	commands  chan *flyontime.Command
	callbacks map[string]*flyontime.Notification
	// messages keeps track of previous messages
	messages map[messageKey]*slack.MessageEvent
}

func (s *Notifier) init() {
	s.initOnce.Do(func() {
		s.slack = slack.New(s.Token)
		s.commands = make(chan *flyontime.Command)
		s.callbacks = make(map[string]*flyontime.Notification)
		s.messages = make(map[messageKey]*slack.MessageEvent)
	})
}

func (s *Notifier) Commands() <-chan *flyontime.Command {
	s.init()

	go func() {
		rtm := s.slack.NewRTM()
		go rtm.ManageConnection()

		for msg := range rtm.IncomingEvents {
			switch ev := msg.Data.(type) {
			case *slack.MessageEvent:
				s.handleMessageEvent(ev)
			}
		}
	}()

	return s.commands
}

func (s *Notifier) handleMessageEvent(m *slack.MessageEvent) {
	if m.SubType == "message_replied" {
		totalReplies := len(m.SubMessage.Replies)
		lastReply := m.SubMessage.Replies[totalReplies-1]
		reply, ok := s.messages[messageKey{lastReply.User, lastReply.Timestamp}]
		if !ok {
			return
		}
		atts := m.SubMessage.Attachments
		if len(atts) > 0 && atts[0].CallbackID != "" {
			s.handleReply(atts[0].CallbackID, reply.Text, m.SubMessage.ThreadTimestamp)
			return
		}
		return
	}
	// TODO(borshukov): We need some way to get rid of old messages.
	s.messages[messageKey{m.User, m.Timestamp}] = m
	return
}

func (s *Notifier) handleReply(callbackID, text, threadTimestamp string) {
	n, ok := s.callbacks[callbackID]
	if !ok {
		return
	}
	cmd, args := parseCommand(text)

	go func() {
		responses := make(chan string)
		// Send the command.
		s.commands <- &flyontime.Command{
			Name:      cmd,
			Args:      args,
			Job:       &n.Job,
			Responses: responses,
		}
		// And post each response as a message.
		for r := range responses {
			s.slack.PostMessage(s.ChannelID, r, slack.PostMessageParameters{
				ThreadTimestamp: threadTimestamp,
			})
		}
	}()
}

func (s *Notifier) Notify(n *flyontime.Notification) error {
	s.init()
	callbackID := uuid.NewV4().String()
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
				CallbackID: callbackID,
			},
		},
	}
	_, _, err := s.slack.PostMessage(s.ChannelID, "", p)
	if err != nil {
		return err
	}
	s.callbacks[callbackID] = n
	return nil
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

type messageKey struct {
	User      string
	Timestamp string
}

func parseCommand(text string) (string, []string) {
	s := strings.Split(text, " ")
	switch len(s) {
	case 0:
		return "", nil
	case 1:
		return s[0], nil
	default:
		return s[0], s[1:]
	}

}
