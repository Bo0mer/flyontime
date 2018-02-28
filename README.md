# flyontime

Command flyontime implements interactive Slack/Mattermost bot that monitors
Concourse CI jobs and sends notifications on significant events.

Notifications are sent for on following job state transitions:
* job fails
* job succeeds after failing at least once
* job errors
* job is aborted

Additionally, each notification message supports a set of commands for taking
further actions. To invoke a specific command, just reply to the notification
message. The following commands are currently supported:

* `rerun`, `retry` - Rerun the job and post its new status.
* `mute [duration]`, `silence [duration]` - Mute notifications for this
  particular job for `duration` (e.g. `mute 30m`).
* `pause`, `stop` - Pause the pipeline which the job is part of.
* `unapuse`, `play` - Pause the pipeline which the job is part of.

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
  -concourse-password="": Concourse Password
  -concourse-team="main": Concourse Team
  -concourse-url="http://localhost:8080": Concourse URL
  -concourse-username="": Concourse Username
  -mattermost-channel-id="": Mattermost channel id for sending alerts
  -mattermost-token="": Mattermost token for sending alerts
  -mattermost-url="": Mattermost channel id for sending alerts
  -slack-channel-id="": Slack channel id for sending alerts
  -slack-token="": Slack token for sending alerts
  -verbose=false: Enable verbose output
```
