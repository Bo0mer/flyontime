package slacker

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"code.cloudfoundry.org/lager"
	"github.com/Bo0mer/flyontime/pkg/flyontime"
	"github.com/lunixbochs/vtclean"
	"github.com/nlopes/slack"
	uuid "github.com/satori/go.uuid"
)

type Notifier struct {
	Token     string
	ChannelID string
	Logger    lager.Logger

	initOnce sync.Once
	slack    *slack.Client
	selfID   string

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
		if s.Logger == nil {
			s.Logger = lager.NewLogger("")
		}
		s.updateBotUser()
	})
}

func (s *Notifier) Commands() <-chan *flyontime.Command {
	s.init()

	go func() {
		rtm := s.slack.NewRTM()
		go rtm.ManageConnection()

		for msg := range rtm.IncomingEvents {
			switch ev := msg.Data.(type) {
			case *slack.ConnectedEvent:
				s.selfID = ev.Info.User.ID
			case *slack.MessageEvent:
				s.handleMessageEvent(ev)
			}
		}
	}()

	return s.commands
}

func (s *Notifier) handleMessageEvent(m *slack.MessageEvent) {
	// There are three distinct type of messages that are handled:
	// 1) IM (direct) messages
	// 2) Replies to posted notifications
	// 3) Mentions in the configured channel

	if s.isIM(m.Channel) && m.User != "" && m.BotID == "" {
		s.handleDirectMessage(m)
		return
	}
	if m.Channel != s.ChannelID {
		return
	}
	if m.SubType == "message_replied" {
		s.handleReplyMessage(m)
		return
	}
	if strings.HasPrefix(m.Msg.Text, fmt.Sprintf("<@%s> ", s.selfID)) {
		s.handleMentionMessage(m)
		return
	}

	// TODO(borshukov): We need some way to get rid of old messages.
	s.messages[messageKey{m.User, m.Timestamp}] = m
	return
}

func (s *Notifier) handleDirectMessage(m *slack.MessageEvent) {
	cmd, args := parseCommand(m.Msg.Text)
	s.run(&flyontime.Command{Name: cmd, Args: args}, s.replyToIM(m.Channel))
}

func (s *Notifier) handleReplyMessage(m *slack.MessageEvent) {
	totalReplies := len(m.SubMessage.Replies)
	lastReply := m.SubMessage.Replies[totalReplies-1]
	reply, ok := s.messages[messageKey{lastReply.User, lastReply.Timestamp}]
	if !ok {
		return
	}
	atts := m.SubMessage.Attachments
	if len(atts) == 0 || atts[0].CallbackID == "" {
		return
	}
	callbackID := atts[0].CallbackID
	n, ok := s.callbacks[callbackID]
	if !ok {
		return
	}
	cmd, args := parseCommand(reply.Text)
	s.run(&flyontime.Command{Name: cmd, Args: args, Job: &n.Job}, s.replyToThread(m.SubMessage.ThreadTimestamp))
}

func (s *Notifier) handleMentionMessage(m *slack.MessageEvent) {
	words := strings.Split(strings.TrimSpace(m.Msg.Text), " ")
	if len(words) < 2 {
		// TODO(borshukov): Log.
		return
	}

	cmd, args := words[1], words[2:]
	s.run(&flyontime.Command{Name: cmd, Args: args}, s.replyToThread(m.ThreadTimestamp))
}

func (s *Notifier) run(c *flyontime.Command, reply replyFunc) {
	go func() {
		responses := make(chan string)
		c.Responses = responses
		// Send the command.
		s.commands <- c
		// And post each response as a message.
		for r := range responses {
			reply(r)
		}
	}()
}

type replyFunc func(reply string)

func (s *Notifier) replyToThread(ts string) replyFunc {
	return func(reply string) {
		s.slack.PostMessage(s.ChannelID, reply, slack.PostMessageParameters{
			ThreadTimestamp: ts,
			Markdown:        true,
		})
	}
}

func (s *Notifier) replyToIM(channelID string) replyFunc {
	return func(reply string) {
		s.slack.PostMessage(channelID, reply, slack.PostMessageParameters{
			Markdown: true,
		})
	}
}

func (s *Notifier) Notify(ctx context.Context, n *flyontime.Notification) error {
	s.init()
	callbackID := uuid.NewV4().String()
	p := slack.PostMessageParameters{
		Attachments: []slack.Attachment{
			slack.Attachment{
				Color:      colorFor(n.Severity),
				AuthorName: "Concourse",
				AuthorIcon: "https://concourse.ci/favicon.ico",
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

func (s *Notifier) isIM(channelID string) bool {
	logger := s.Logger.Session("is-im")
	// TODO(borshukov): Cache the result of GetIMChannels.
	ims, err := s.slack.GetIMChannels()
	if err != nil {
		logger.Error("get-im-channels.fail", err)
		return false
	}

	for _, im := range ims {
		if im.ID == channelID {
			return true
		}
	}
	return false
}

func (s *Notifier) updateBotUser() {
	logger := s.Logger.Session("update-user")
	path, err := tempLogoFile()
	if err != nil {
		logger.Error("write-tmp-user-photo-file.fail", err)
		return
	}
	if err := s.slack.SetUserPhoto(path, slack.UserSetPhotoParams{}); err != nil {
		logger.Error("set-user-photo.fail", err)
		return
	}

	if err := os.RemoveAll(path); err != nil {
		logger.Error("cleanup-tmp-user-photo-file.fail,", err)
		return
	}
	logger.Info("done")
}

func tempLogoFile() (string, error) {
	fd, err := ioutil.TempFile("", "logo.png")
	if err != nil {
		return "", err
	}
	defer fd.Close()
	_, err = fd.Write(concourseLogoPNG[:])

	return fd.Name(), nil
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
	return fmt.Sprintf("```\n%s\n```", vtclean.Clean(code, false))
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
