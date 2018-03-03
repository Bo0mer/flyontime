package flyontime

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
	"golang.org/x/oauth2"
)

type AutoPilot struct {
	concourse.Client
	concourse.Team
	Logger       lager.Logger
	PollInterval time.Duration
}

func NewAutoPilot(concourseURL, team, username, password string, logger lager.Logger) (*AutoPilot, error) {
	c, err := newConcourseClient(concourseURL, team, username, password)
	if err != nil {
		return nil, err
	}

	return &AutoPilot{
		Client:       c,
		Team:         c.Team(team),
		PollInterval: 4 * time.Second,
		Logger:       logger,
	}, nil
}

func (p *AutoPilot) FinishedBuilds(ctx context.Context) <-chan atc.Build {
	c := make(chan atc.Build)
	go func() {
		logger := p.Logger.Session("finished-builds")
		t := time.NewTicker(p.PollInterval)
		defer func() {
			t.Stop()
			close(c)
		}()
		_, pg, err := p.Builds(concourse.Page{Limit: 1})
		if err != nil {
			logger.Session("init").Error("fail", err)
			return
		}
		lastSeen := pg.Next.Since
		var builds []atc.Build
		for {
			select {
			case <-t.C:
				logger := logger.Session("get-latest")
				builds, pg, err = p.Builds(concourse.Page{Until: lastSeen, Limit: 100})
				if err != nil {
					logger.Error("fail", err)
					continue
				}
				if len(builds) == 0 || pg.Next == nil {
					// No new builds.
					continue
				}
				// If there are more than 100 new builds.
				if last := pg.Next.Since; last-lastSeen > 100 {
					lastSeen = lastSeen + 100
				} else {
					lastSeen = pg.Next.Since
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
				logger.Info("exit")
				return
			}
		}
	}()
	return c
}

func (p *AutoPilot) ListPipelines() ([]atc.Pipeline, error) {
	if p.Team != nil {
		return p.Team.ListPipelines()
	}
	return p.Client.ListPipelines()
}

func newConcourseClient(url, team, username, password string) (concourse.Client, error) {
	c := concourse.NewClient(url, authenticatedClient(username, password), false)

	transport := &oauth2.Transport{
		Source: &teamTokenSource{c.Team(team)},
		Base:   baseTransport(),
	}

	return concourse.NewClient(url, &http.Client{Transport: transport}, false), nil
}

type teamTokenSource struct {
	team concourse.Team
}

func (ts *teamTokenSource) Token() (*oauth2.Token, error) {
	token, err := ts.team.AuthToken()
	if err != nil {
		return nil, err
	}

	return &oauth2.Token{
		TokenType:   token.Type,
		AccessToken: token.Value,
	}, nil
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

//go:generate counterfeiter . concourseClient
//go:generate counterfeiter . team

type concourseClient interface {
	concourse.Client
}

type team interface {
	concourse.Team
}
