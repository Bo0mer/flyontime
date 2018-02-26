package mattermost

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/Bo0mer/flyontime/pkg/flyontime"
	"github.com/mattermost/mattermost-server/model"
	"github.com/pkg/errors"
)

type Notifier struct {
	API         string
	Token       string
	ChannelID   string // provide either ChannelId or TeamName and ChannelName
	TeamName    string
	ChannelName string

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
		mm.commands = make(chan *flyontime.Command)
		mm.posts = make(map[string]*flyontime.Notification)
		mm.self = self
	})
	return mm.initErr
}

func (mm *Notifier) Commands() <-chan *flyontime.Command {
	mm.init()

	api, err := url.Parse(mm.API)
	if err != nil {
		// TODO(borshukov): Handle error.
		return nil
	}
	if api.Scheme == "https" {
		api.Scheme = "wss"
	} else {
		api.Scheme = "ws"
	}

	go func() {
		ws, err := model.NewWebSocketClient4(api.String(), mm.Token)
		if err != nil {
			// TODO(borshukov): Handle error.
			return
		}
		ws.Listen()

		for ev := range ws.EventChannel {
			if ev.EventType() != "posted" {
				continue
			}
			postJSON, ok := ev.Data["post"].(string)
			if !ok {
				// TODO(borshukov): Handle error.
				continue
			}
			p := new(model.Post)
			if err := json.Unmarshal([]byte(postJSON), p); err != nil {
				// TODO(borshukov): Handle error.
				continue
			}
			if p.UserId == mm.self.Id {
				// Do not reply to self.
				continue
			}

			mm.handleReply(p, p.Message)
		}
	}()

	return mm.commands
}

func (mm *Notifier) handleReply(reply *model.Post, text string) {
	n, ok := mm.posts[reply.ParentId]
	if !ok {
		return
	}
	cmd, args := parseCommand(text)

	go func() {
		responses := make(chan string)
		// Send the command.
		mm.commands <- &flyontime.Command{
			Name:      cmd,
			Args:      args,
			Job:       &n.Job,
			Responses: responses,
		}
		// And post each response as a message.
		for r := range responses {
			// TODO(borshukov): Handle error.
			mm.client.CreatePost(&model.Post{
				Message:   r,
				ChannelId: mm.ChannelID,
				ParentId:  reply.Id,
				RootId:    reply.RootId,
			})
		}
	}()
}

func (mm *Notifier) Notify(n *flyontime.Notification) error {
	if err := mm.init(); err != nil {
		return err
	}
	post := &model.Post{ChannelId: mm.ChannelID}
	post.AddProp("attachments", []*model.SlackAttachment{
		&model.SlackAttachment{
			Color:      colorFor(n.Severity),
			AuthorName: "Concourse",
			AuthorIcon: "",
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
	return fmt.Sprintf("```text\n%s\n```", code)
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
