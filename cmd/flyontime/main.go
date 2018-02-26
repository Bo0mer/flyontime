// Command flyontime monitors Concourse jobs and sends notifications on
// significant events.
package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bo0mer/flyontime/pkg/flyontime"
	"github.com/Bo0mer/flyontime/pkg/mattermost"
	"github.com/Bo0mer/flyontime/pkg/slacker"
	"github.com/concourse/go-concourse/concourse"
	"github.com/namsral/flag"
	"golang.org/x/oauth2"
)

var (
	slackChannelID string
	slackToken     string

	mattermostURL       string
	mattermostChannelID string
	mattermostToken     string

	concourseURL      string
	concourseUsername string
	concoursePassword string
	concourseTeam     string
)

func init() {
	flag.StringVar(&slackChannelID, "slack-channel-id", "", "Slack channel id for sending alerts")
	flag.StringVar(&slackToken, "slack-token", "", "Slack token for sending alerts")

	flag.StringVar(&mattermostURL, "mattermost-url", "", "Mattermost channel id for sending alerts")
	flag.StringVar(&mattermostChannelID, "mattermost-channel-id", "", "Mattermost channel id for sending alerts")
	flag.StringVar(&mattermostToken, "mattermost-token", "", "Mattermost token for sending alerts")

	flag.StringVar(&concourseURL, "concourse-url", "http://localhost:8080", "Concourse URL")
	flag.StringVar(&concourseUsername, "concourse-username", "", "Concourse Username")
	flag.StringVar(&concoursePassword, "concourse-password", "", "Concourse Password")
	flag.StringVar(&concourseTeam, "concourse-team", "main", "Concourse Team")
}

func main() {
	flag.Parse()

	pilot, err := flyontime.NewPilot(concourseURL, concourseTeam, concourseUsername, concoursePassword)
	if err != nil {
		log.Fatal(err)
	}
	nc := chatFromFlags()
	m := flyontime.NewMonitor(pilot, nc, nc)
	go m.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	m.Stop()

	fmt.Printf("Bye\n")
}

type chat interface {
	flyontime.Notifier
	flyontime.Commander
}

func chatFromFlags() (n chat) {
	if slackToken != "" {
		n = &slacker.Notifier{
			Token:     slackToken,
			ChannelID: slackChannelID,
		}
	}
	if mattermostToken != "" {
		n = &mattermost.Notifier{
			API:       mattermostURL,
			Token:     mattermostToken,
			ChannelID: mattermostChannelID,
		}
	}
	return n
}

func newConcourseClient(url, team, username, password string) (concourse.Client, error) {
	c := concourse.NewClient(url, authenticatedClient(username, password), false)

	t := c.Team(team)
	token, err := t.AuthToken()
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate to team: %s", err)
	}

	oAuthToken := &oauth2.Token{
		TokenType:   token.Type,
		AccessToken: token.Value,
	}

	transport := &oauth2.Transport{
		Source: oauth2.StaticTokenSource(oAuthToken),
		Base:   baseTransport(),
	}

	return concourse.NewClient(url, &http.Client{Transport: transport}, false), nil
}

func authenticatedClient(username, password string) *http.Client {
	return &http.Client{
		Transport: &basicAuthTransport{
			username: username,
			password: password,
			base:     baseTransport(),
		},
	}
}

func baseTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		Dial: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).Dial,
		Proxy: http.ProxyFromEnvironment,
	}

}

type basicAuthTransport struct {
	username string
	password string

	base http.RoundTripper
}

func (t basicAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.SetBasicAuth(t.username, t.password)
	return t.base.RoundTrip(r)
}
