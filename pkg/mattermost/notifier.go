package mattermost

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/Bo0mer/flyontime/pkg/flyontime"
	"github.com/mattermost/mattermost-server/model"
)

type Notifier struct {
	API         string
	Token       string
	ChannelID   string // provide either ChannelId or TeamName and ChannelName
	TeamName    string
	ChannelName string

	initOnce sync.Once
	client   *model.Client4
	initErr  error
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

		if mm.ChannelID != "" {
			team, resp := mm.client.GetTeamByName(mm.TeamName, "")
			if resp.Error != nil {
				mm.initErr = resp.Error
				return
			}

			channel, resp := mm.client.GetChannelByName(mm.ChannelName, team.Id, "")
			if resp.Error != nil {
				mm.initErr = resp.Error
				return
			}

			mm.ChannelID = channel.Id
		}
	})
	return mm.initErr
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

	mm.client.CreatePost(post)
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
