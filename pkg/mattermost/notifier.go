package mattermost

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"code.cloudfoundry.org/lager"
	"github.com/Bo0mer/flyontime/pkg/flyontime"
	"github.com/lunixbochs/vtclean"
	"github.com/mattermost/mattermost-server/model"
	"github.com/pkg/errors"
)

type Notifier struct {
	API         string
	Token       string
	ChannelID   string // provide either ChannelId or TeamName and ChannelName
	TeamName    string
	ChannelName string
	Logger      lager.Logger

	initOnce sync.Once
	client   *model.Client4
	self     *model.User
	initErr  error

	commands chan *flyontime.Command
	posts    map[string]*flyontime.Notification // maps post id to notification
}

func (mm *Notifier) init() error {
	if mm.initErr != nil {
		return mm.initErr
	}
	mm.initOnce.Do(func() {
		mm.client = &model.Client4{
			Url:        mm.API,
			ApiUrl:     mm.API + model.API_URL_SUFFIX,
			HttpClient: &http.Client{},
			AuthToken:  mm.Token,
			AuthType:   "bearer",
		}

		if mm.ChannelID == "" {
			team, resp := mm.client.GetTeamByName(mm.TeamName, "")
			if resp.Error != nil {
				mm.initErr = errors.Wrap(resp.Error, "error obtaining team")
				return
			}

			channel, resp := mm.client.GetChannelByName(mm.ChannelName, team.Id, "")
			if resp.Error != nil {
				mm.initErr = errors.Wrap(resp.Error, "error obtaining channel")
				return
			}

			mm.ChannelID = channel.Id
		}

		self, resp := mm.client.GetMe("")
		if resp.Error != nil {
			mm.initErr = errors.Wrap(resp.Error, "error obtaining bot info")
			return
		}
		mm.updateBotUser(self)

		mm.commands = make(chan *flyontime.Command)
		mm.posts = make(map[string]*flyontime.Notification)
		mm.self = self
	})
	return mm.initErr
}

func (mm *Notifier) Commands() <-chan *flyontime.Command {
	mm.init()

	logger := mm.Logger.Session("commands")
	api, err := url.Parse(mm.API)
	if err != nil {
		logger.Error("parse-api-url.fail", err)
		return nil
	}
	if api.Scheme == "https" {
		api.Scheme = "wss"
	} else {
		api.Scheme = "ws"
	}

	go func() {
		for {
			ws, err := model.NewWebSocketClient4(api.String(), mm.Token)
			if err != nil {
				logger.Error("websocket-connect.fail", err)
				return
			}

			ws.Listen()

			for ev := range ws.EventChannel {
				if ev.EventType() != model.WEBSOCKET_EVENT_POSTED {
					logger.Debug("skip-message")
					continue
				}
				postJSON, ok := ev.Data["post"].(string)
				if !ok {
					logger.Error("get-post-data.fail", err)
					continue
				}
				p := model.PostFromJson(strings.NewReader(postJSON))
				if p.UserId == mm.self.Id {
					// Do not reply to self.
					continue
				}

				mm.handlePost(logger, p)
			}

			if ws.ListenError != nil {
				logger.Error("fail-will-retry", ws.ListenError)
				continue
			}
			logger.Info("exit")
			break
		}
	}()

	return mm.commands
}

func (mm *Notifier) handlePost(logger lager.Logger, post *model.Post) {
	if n, ok := mm.posts[post.ParentId]; ok {
		mm.handleReply(logger.Session("handle-reply"), post, n)
		return
	}

	if strings.HasPrefix(post.Message, fmt.Sprintf("@%s ", strings.ToLower(mm.self.Username))) {
		mm.handleMention(logger.Session("handle-mention"), post)
		return
	}

	if mm.isDirectMessage(logger, post) && post.UserId != mm.self.Id {
		mm.handleDirectMessage(logger.Session("handle-dm"), post)
		return
	}
}

func (mm *Notifier) handleReply(logger lager.Logger, reply *model.Post, to *flyontime.Notification) {
	cmd, args := parseCommand(reply.Message)

	replyFunc := mm.replyToThread(reply.Id, reply.RootId)
	mm.run(logger, &flyontime.Command{Name: cmd, Args: args, Job: &to.Job}, replyFunc)
}

func (mm *Notifier) handleMention(logger lager.Logger, post *model.Post) {
	words := strings.Split(strings.TrimSpace(post.Message), " ")
	if len(words) < 2 {
		logger.Info("missing-command")
		return
	}

	cmd, args := words[1], words[2:]
	mm.run(logger, &flyontime.Command{Name: cmd, Args: args}, mm.replyToChannel(post.ChannelId))
}

func (mm *Notifier) handleDirectMessage(logger lager.Logger, dm *model.Post) {
	cmd, args := parseCommand(dm.Message)

	mm.run(logger, &flyontime.Command{Name: cmd, Args: args}, mm.replyToChannel(dm.ChannelId))
}

func (mm *Notifier) run(logger lager.Logger, c *flyontime.Command, reply replyFunc) {
	go func() {
		responses := make(chan string)
		c.Responses = responses

		// Send the command.
		mm.commands <- c

		// And post each response as a message.
		for r := range responses {
			if err := reply(r); err != nil {
				logger.Error("reply.fail", err)
			}
		}
	}()
}

type replyFunc func(string) error

func (mm *Notifier) replyToThread(parentID, rootID string) replyFunc {
	return func(reply string) error {
		_, resp := mm.client.CreatePost(&model.Post{
			Message:   reply,
			ChannelId: mm.ChannelID,
			ParentId:  parentID,
			RootId:    rootID,
		})
		// NOTE(borshukov): resp.Error has a concrete type, thus it is always
		// non-nil interface.
		if resp.Error == nil {
			return nil
		}
		return resp.Error
	}
}

func (mm *Notifier) replyToChannel(cid string) replyFunc {
	return func(reply string) error {
		_, resp := mm.client.CreatePost(&model.Post{
			Message:   reply,
			ChannelId: cid,
		})
		// NOTE(borshukov): resp.Error has a concrete type, thus it is always
		// non-nil interface.
		if resp.Error == nil {
			return nil
		}
		return resp.Error
	}
}

func (mm *Notifier) isDirectMessage(logger lager.Logger, post *model.Post) bool {
	// TODO(borshukov): Cache known direct channels.
	channel, resp := mm.client.GetChannel(post.ChannelId, "")
	if resp.Error != nil {
		logger.Error("get-channel-details.fail", resp.Error)
		return false
	}
	return channel.Type == model.CHANNEL_DIRECT
}

func (mm *Notifier) Notify(ctx context.Context, n *flyontime.Notification) error {
	if err := mm.init(); err != nil {
		return err
	}
	post := &model.Post{ChannelId: mm.ChannelID}
	post.AddProp("attachments", []*model.SlackAttachment{
		&model.SlackAttachment{
			Color:      colorFor(n.Severity),
			AuthorName: "Concourse",
			AuthorIcon: "https://concourse.ci/favicon.ico",
			Title:      n.Title,
			TitleLink:  n.DashboardLink,
			Text:       formatCode(n.JobOutput),
		},
	})

	p, resp := mm.client.CreatePost(post)
	if resp.Error != nil {
		return resp.Error
	}
	// TODO(borshukov): Get rid of old posts.
	mm.posts[p.Id] = n
	return nil
}

func (mm *Notifier) updateBotUser(user *model.User) {
	logger := mm.Logger.Session("update-user")
	// TODO(borshukov): This could be configurable.
	user.Username = "Concourse"
	user.FirstName = "Continuous"
	user.LastName = "Integration"

	_, resp := mm.client.UpdateUser(user)
	if resp.Error != nil {
		logger.Error("fail", resp.Error)
		return
	}

	ok, resp := mm.client.SetProfileImage(user.Id, concourseLogoPNG[:])
	if !ok || resp.Error != nil {
		logger.Error("set-profile-image.fail", resp.Error)
	}

	logger.Info("done")
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
	return fmt.Sprintf("```text\n%s\n```", vtclean.Clean(code, false))
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
