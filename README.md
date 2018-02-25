# flyontime

Command flyontime monitors Concourse jobs and sends notifications on significant
events.

## Usage

Configuration could be provided both from environment variables and as
command-line arguments to the `flyontime` command, e.g.

```
export CONCOURSE_PASSWORD="s3cr3t-password"
export SLACK_TOKEN="slack-t0k3n"
flyontime -concourse-username="Bob" -slack-channel-id="42"
```

Full usage help can be printed by providing the `--help` flag:

```
Usage of flyontime:
  -concourse-password string
    	Concourse Password
  -concourse-team string
    	Concourse Team (default "main")
  -concourse-url string
    	Concourse URL (default "http://localhost:8080")
  -concourse-username string
    	Concourse Username
  -mattermost-channel-id string
    	Mattermost channel id for sending alerts
  -mattermost-token string
    	Mattermost token for sending alerts
  -slack-channel-id string
    	Slack channel id for sending alerts
  -slack-token string
    	Slack token for sending alerts
```
