package flyontime

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
	"golang.org/x/oauth2"
)

var (
	pollInterval = 4 * time.Second
)

type Pilot struct {
	concourse.Client
	concourse.Team

	pollInterval time.Duration
	log          lager.Logger
}

func NewPilot(concourseURL, team, username, password string) (*Pilot, error) {
	log := lager.NewLogger("flyontime")
	log.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	c, err := newConcourseClient(concourseURL, team, username, password)
	if err != nil {
		return nil, err
	}

	return &Pilot{
		Client:       c,
		Team:         c.Team(team),
		pollInterval: 4 * time.Second,
		log:          log,
	}, nil
}

func (p *Pilot) FinishedBuilds(ctx context.Context) <-chan atc.Build {
	c := make(chan atc.Build)
	go func() {
		log := p.log
		t := time.NewTicker(p.pollInterval)
		defer func() {
			t.Stop()
			close(c)
		}()
		_, pg, err := p.Builds(concourse.Page{Limit: 1})
		if err != nil {
			return
		}
		lastSeen := pg.Next.Since
		var builds []atc.Build
		for {
			select {
			case <-t.C:
				log := log.Session("recheck")
				log.Debug("start")
				builds, pg, err = p.Builds(concourse.Page{Until: lastSeen, Limit: 100})
				if err != nil {
					log.Error("fail", err)
					continue
				}
				if pg.Next == nil {
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
				return
			}
		}
	}()
	return c
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
