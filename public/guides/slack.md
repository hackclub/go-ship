# Making a Slack bot

<img src="/public/guides/assets/slack.png" alt="Slack Logo" width="250">

In this guide, we'll be creating a simple Slack bot using the slack-go/slack library, an API client for slack in Go.

We'll be implementing:

- Listening to messages in a channel and responding to them
- Slash Commands
- Sending interactive messages with a button to invite people to your channel

## Getting Started

Let's start by creating a new folder + initializing the Go module and installing the slack-go/slack library.

```bash
mkdir slack-bot
cd slack-bot
go mod init slack-bot
go get github.com/slack-go/slack
```

Then, create a `main.go` file and we'll start creating our slack bot.

```go
package main
