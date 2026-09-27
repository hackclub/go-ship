# Making a Slack bot using slack-go/slack

<img src="/public/guides/assets/slack.png" alt="Slack Logo" width="250">

[slack-go/slack](https://github.com/slack-go/slack) is a Go library for interacting with Slack's API. It lets us send messages, receive events, respond to slash commands, and more.

In this guide, we'll be making a simple bot that responds to mentions in a thread, has a `/goship` slash command, and uses Block Kit to display messages. We'll be using Socket Mode, which lets us receive events through a WebSocket connection instead of having to host a public HTTP endpoint.

## Creating a Slack app

First, we need to create an app and give it the permissions needed to send messages and receive events.

1. Head to <https://api.slack.com/apps>, click Create New App, Blank App, click continue, give it a name, and select the Hack Club workspace
2. Go to the Socket Mode tab and enable it. Create an app-level token with the `connections:write` scope, and copy the token starting with `xapp-`.
3. Next, head to OAuth & Permissions. Under Bot Token Scopes, add the following scopes:
   - `app_mentions:read`: Receive events when somebody mentions the bot.
   - `chat:write`: Send messages as the bot.
   - `commands`: Create and use slash commands. Slack may add this one automatically when we create the command, but make sure it's there.
4. In Event Subscriptions, enable events and subscribe to the bot event `app_mention`. Since we're using Socket Mode, we don't need to provide a Request URL.
5. In Slash Commands, create a command called `/goship`. You can give it a description such as "Say hello from Go".
6. Install the app into the workspace via the Install App tab, then copy the Bot User OAuth Token, which starts with `xoxb-`.

Now, we should have two different tokens:

- `xapp-` is the app-level token, which we'll use to connect to Slack through Socket Mode.
- `xoxb-` is the bot token, which we'll use for sending messages and other Slack API requests.

Don't share these tokens or commit them to Git! If you change the bot's scopes later, you'll need to reinstall the app to grant the new permissions.

Finally, invite your bot to a test channel using `/invite @YourBot` or by pinging the bot, then clicking invite them on slackbot's reply. You can use #bot-spam, or create your own channel.

## Setting up the project

Assuming you already have Go installed, let's make a new folder and initialize the Go module:

```bash
mkdir slack-bot
cd slack-bot
go mod init slack-bot
```

Next, install `slack-go/slack` and `godotenv` (which we'll use to load our tokens from a `.env` file):

```bash
go get github.com/slack-go/slack
go get github.com/joho/godotenv
```

Create a `.env` file in your project directory, with the tokens you copied earlier:

```dotenv
SLACK_APP_TOKEN="xapp-your-app-token"
SLACK_BOT_TOKEN="xoxb-your-bot-token"
```

And make a `.gitignore` file, so we don't accidentally upload our tokens to Git:

```gitignore
.env
!.env.example
```

I highly recommend adding a .env.example to your projects as it can help future you by showing what tokens you need, and whoever else may work on the same project.

Obviously, replace the example tokens with your actual ones. The Slack library includes the `slackevents` and `socketmode` packages we'll use later, so we don't need to install anything else.

## Socket mode time

Let's start off with a basic `main.go` that loads our tokens, connects to Slack, and prints any events or slash commands that we receive. This way, we can make sure everything works before adding the actual bot functionality.

Create `main.go` and add the following code:

```go
package main

import (
    "log"
    "os"

    "github.com/joho/godotenv"
    "github.com/slack-go/slack"
    "github.com/slack-go/slack/slackevents"
    "github.com/slack-go/slack/socketmode"
)


func main() {
    // Load the tokens from the .env file into the environment
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found, using environment variables")
    }

    appToken := os.Getenv("SLACK_APP_TOKEN")
    botToken := os.Getenv("SLACK_BOT_TOKEN")
    if appToken == "" || botToken == "" {
        log.Fatal("Set SLACK_APP_TOKEN and SLACK_BOT_TOKEN first")
    }

    // Initialize the Slack API and Socket Mode clients.
    api := slack.New(botToken, slack.OptionAppLevelToken(appToken))
    client := socketmode.New(api)

    // Run our event listener in a goroutine, since client.Run() blocks.
    go func() {
        for evt := range client.Events {
            switch evt.Type {
            case socketmode.EventTypeConnected:
                log.Println("Connected to Slack!")

            case socketmode.EventTypeEventsAPI:
                // Tell Slack that we've received the event.
                if err := client.Ack(*evt.Request); err != nil {
                    log.Println("Failed to acknowledge event:", err)
                }

                event, ok := evt.Data.(slackevents.EventsAPIEvent)
                if !ok {
                    continue
                }
                log.Printf("Received event: %T\n", event.InnerEvent.Data)

            case socketmode.EventTypeSlashCommand:
                // Slash commands need to be acknowledged as well (just like discord.js)
                cmd, ok := evt.Data.(slack.SlashCommand)
                if !ok {
                    client.Ack(*evt.Request)
                    continue
                }
                log.Println("Received command:", cmd.Command)
                if err := client.Ack(*evt.Request, map[string]any{
                    "response_type": "ephemeral",
                    "text":          "Hello from Go!",
                }); err != nil {
                    log.Println("Failed to respond to command:", err)
                }
            }
        }
    }()

    log.Println("Connecting to Slack...")
    if err := client.Run(); err != nil {
        log.Fatal(err)
    }
}
```

Now, run the bot from your project directory:

```bash
go run .
```

Mention the bot in the channel you invited it to. You should see that an event was received in your terminal. Try `/goship` as well, and it should print the command name and display "Hello from Go!" privately in Slack.

---

Let's break down the important parts of the code.

- `godotenv.Load()` loads our `.env` file, so `os.Getenv` can read the tokens. If you already set them as environment variables yourself, the code can use those too.
- `slack.New()` creates the API client using our bot token. `slack.OptionAppLevelToken()` provides the app token for Socket Mode.
- `socketmode.New(api)` creates the Socket Mode client and handles the WebSocket connection for us.
- `client.Events` is a Go channel containing incoming events. Our `for` loop keeps receiving events as long as the bot is running.
- `evt.Type` tells us what kind of event we've received. We use a type assertion on `evt.Data` to access its actual contents.
- `client.Run()` connects to Slack and keeps running, which is why we put the event listener inside a goroutine.

One important thing is `client.Ack(*evt.Request)`. It acknowledges the event, telling Slack that we've received it. This doesn't actually send a message to the channel, and we'll still have to do that ourselves. Slack expects acknowledgements quickly (within about 3 seconds), so don't put slow API calls or other work before them.

## Responding to pings in a thread

Alright, now that we've got a working connection, let's make the bot actually respond to mentions.

The `app_mention` event gives us the user who mentioned the bot, the channel it happened in, the message's timestamp, and other information. Slack wraps it inside an Events API event, so we need to access `event.InnerEvent.Data` to get it.

Replace the `socketmode.EventTypeEventsAPI` case in our event loop with this:

```go
case socketmode.EventTypeEventsAPI:
    // Acknowledge the event before doing any other work.
    if err := client.Ack(*evt.Request); err != nil {
        log.Println("Failed to acknowledge event:", err)
    }

    event, ok := evt.Data.(slackevents.EventsAPIEvent)
    if !ok || event.Type != slackevents.CallbackEvent {
        continue
    }

    // Check if the event is an app mention.
    mention, ok := event.InnerEvent.Data.(*slackevents.AppMentionEvent)
    if !ok || mention.BotID != "" || mention.User == "" {
        continue
    }

    go handleMention(api, mention)
```

We check whether the event contains an `AppMentionEvent`, ignoring everything else. The `BotID` check also helps us ignore bot-authored messages.

Now, let's make a separate function to handle mentions. Add this anywhere outside the `main()` function:

```go
func handleMention(api *slack.Client, mention *slackevents.AppMentionEvent) {
    threadTS := mention.ThreadTimeStamp
    if threadTS == "" {
        threadTS = mention.TimeStamp
    }

    _, _, err := api.PostMessage(
        mention.Channel,
        slack.MsgOptionText("Hey <@"+mention.User+">! Go Ship!", false),
        slack.MsgOptionTS(threadTS),
    )
    if err != nil {
        log.Println("Failed to reply to mention:", err)
    }
}
```

Now, if somebody mentions the bot, it should respond with a message in the same thread. But how does it know which thread to use?

Slack uses message timestamps (`ts`) as message identifiers. When we want to post a reply, we pass the parent message's timestamp as `thread_ts`.

```go
threadTS := mention.ThreadTimeStamp
if threadTS == "" {
    threadTS = mention.TimeStamp
}
```

- `mention.ThreadTimeStamp` is the parent timestamp if the mention is already inside a thread.
- `mention.TimeStamp` is the timestamp of the mention itself. If there isn't an existing thread, we use it to create one under the message that mentioned us.
- `slack.MsgOptionTS(threadTS)` passes that timestamp to Slack when posting the response. Without it, we'd just send a new message to the channel.

And let's break down `PostMessage` as well:

```go
_, _, err := api.PostMessage(
    mention.Channel,                         // The channel to send the message in.
    slack.MsgOptionText("Hello!", false),   // The message text.
    slack.MsgOptionTS(threadTS),              // Which thread to reply to.
)
```

`PostMessage` returns the channel ID, the new message's timestamp, and an error. We don't need the first two for this example, which is why we're using `_`, but we should still check the error in case Slack rejects the message.

You might also notice the `<@USER_ID>` syntax in our actual message. That's how Slack mentions a user in a message, so our reply will ping the person who mentioned the bot.

We're using `go handleMention(...)` to run the function in a separate goroutine. This prevents a slow `PostMessage` request from blocking the event loop while other events are coming in. If you make a larger bot later on, you'll also want to account for retried or duplicate events so they don't perform the same action twice.

---

## Slash Commands

Next up, let's make the `/goship` command do something more useful. Slash commands are commands you type into Slack, and we already registered ours in the app settings earlier.

When somebody runs it, Slack sends us a `slack.SlashCommand` containing a few useful fields:

- `cmd.Command`: The command name, such as `/goship`.
- `cmd.Text`: Whatever was typed after it. For example, `/goship hello` gives us `hello`.
- `cmd.UserID`: The user who ran the command.
- `cmd.ChannelID`: The channel where the command was used.
- `cmd.ResponseURL`: A URL that can be used to send a response later.

Let's make the bot respond differently depending on what someone types after `/goship`.

First, add `"strings"` to your imports, then replace the `socketmode.EventTypeSlashCommand` case with this:

```go
case socketmode.EventTypeSlashCommand:
    cmd, ok := evt.Data.(slack.SlashCommand)
    if !ok {
        client.Ack(*evt.Request)
        continue
    }

    if cmd.Command != "/goship" {
        client.Ack(*evt.Request)
        continue
    }

    // Check the command's arguments, ignoring case and extra spaces.
    var response string
    switch strings.ToLower(strings.TrimSpace(cmd.Text)) {
    case "hello":
        response = "Hello from Go!"
    case "help":
        response = "Try /goship or /goship hello."
    default:
        response = "Go Ship! Your slash command works."
    }

    if err := client.Ack(*evt.Request, map[string]any{
        "response_type": "ephemeral",
        "text":          response,
    }); err != nil {
        log.Println("Failed to respond to command:", err)
    }
```

Now try `/goship`, `/goship hello`, and `/goship help`. Each should display a different message.

The `map[string]any` we pass to `Ack` is the response Slack displays for the command. The `response_type` field controls who can see it:

- `ephemeral`: Only the person who ran the command can see the response.
- `in_channel`: The response is visible to the channel.

Slash commands are a bit different from normal messages. For a mention, we acknowledge the event and then call `api.PostMessage()` to send our reply. For a slash command, we can include the response directly in `client.Ack()`, as we did here.

Make sure to acknowledge slash commands within about 3 seconds too. If you need to do something that takes longer, acknowledge the command first, then use the response URL or Slack's Web API to send the result later. Also, custom slash commands can't be invoked from inside a message thread.

## Block Kit

Plain text works, but what if we want to make our messages look a little nicer? That's where [Block Kit](https://docs.slack.dev/block-kit/) comes in.

Block Kit lets us combine things such as headings, sections, dividers, buttons, and more into a single message. For this example, we'll make our `/goship` command return a heading, some text, and a divider.

Add the following code inside the slash command case, just before our `client.Ack()` call:

```go
blocks := []slack.Block{
    slack.NewHeaderBlock(
        slack.NewTextBlockObject(slack.PlainTextType, "Go Ship!", false, false),
    ),
    slack.NewSectionBlock(
        slack.NewTextBlockObject(
            slack.MarkdownType,
            "*Go Ship!* " + response,
            false, false,
        ),
        nil, nil,
    ),
    slack.NewDividerBlock(),
}
```

Let's break it down.

- `[]slack.Block` is a slice of blocks. Slack displays them in the order we add them.
- `NewHeaderBlock` creates a heading. Headers only support plain text, which is why we're using `slack.PlainTextType`.
- `NewSectionBlock` creates a section for our message's contents. The two `nil` arguments mean we're not using any additional fields or accessory elements.
- `NewDividerBlock` adds a horizontal separator.
- `NewTextBlockObject` creates the text inside a block. It takes the text type, the contents, and two additional options which we can leave as `false` for now.

Slack uses its own version of Markdown, called `mrkdwn`. For example, `*bold*` makes text bold, `_italic_` makes it italic, and `<https://go.dev|Go>` creates a link with the label "Go". It's similar to normal Markdown, but not exactly the same.

Now, modify the acknowledgement in our slash command case to include the blocks:

```go
if err := client.Ack(*evt.Request, map[string]any{
    "response_type": "ephemeral",
    "text":          response,
    "blocks":        blocks,
}); err != nil {
    log.Println("Failed to respond to command:", err)
}
```

Run the bot again, and try `/goship` and `/goship hello`. It should now show the Block Kit message instead of only plain text, and the section text should change depending on the argument. We used `response` from our previous example instead of hardcoding another message.

We still include the top-level `text` field as fallback text for notifications and accessibility. It's a good idea to provide it even when you're using blocks.

---

Block Kit isn't limited to slash commands either! We can send the same blocks in a normal message using `PostMessage`:

```go
_, _, err := api.PostMessage(
    "YOUR_CHANNEL_ID",
    slack.MsgOptionText("Go Ship!", false),
    slack.MsgOptionBlocks(blocks...),
)
if err != nil {
    log.Println("Failed to send message:", err)
}
```

Replace `YOUR_CHANNEL_ID` with the channel you want to send the message to, and make sure the bot has permission to post there. The `...` expands our block slice into individual arguments for `MsgOptionBlocks`.

You can experiment with different layouts in Slack's [Block Kit Builder](https://app.slack.com/block-kit-builder/) without having to write Go code for every change. The builder uses JSON, while slack-go gives us functions such as `NewSectionBlock` to construct the same blocks directly.

You can also add interactive buttons using `NewActionBlock` and `NewButtonBlockElement`. If you do, you'll need to enable Interactivity in the Slack app settings and handle `socketmode.EventTypeInteractive` events. Displaying a button alone won't make it do anything when clicked.

## Full example

Here's what our complete `main.go` should look like after making all the changes above. You can compare it with your own file if something isn't working.

```go
package main

import (
    "log"
    "os"
    "strings"

    "github.com/joho/godotenv"
    "github.com/slack-go/slack"
    "github.com/slack-go/slack/slackevents"
    "github.com/slack-go/slack/socketmode"
)

func main() {
    // Load our tokens from the .env file.
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found, using environment variables")
    }

    appToken := os.Getenv("SLACK_APP_TOKEN")
    botToken := os.Getenv("SLACK_BOT_TOKEN")
    if appToken == "" || botToken == "" {
        log.Fatal("Set SLACK_APP_TOKEN and SLACK_BOT_TOKEN first")
    }

    api := slack.New(botToken, slack.OptionAppLevelToken(appToken))
    client := socketmode.New(api)

    go func() {
        for evt := range client.Events {
            switch evt.Type {
            case socketmode.EventTypeConnected:
                log.Println("Connected to Slack!")

            case socketmode.EventTypeEventsAPI:
                // Acknowledge the event before doing any other work.
                if err := client.Ack(*evt.Request); err != nil {
                    log.Println("Failed to acknowledge event:", err)
                }

                event, ok := evt.Data.(slackevents.EventsAPIEvent)
                if !ok || event.Type != slackevents.CallbackEvent {
                    continue
                }

                mention, ok := event.InnerEvent.Data.(*slackevents.AppMentionEvent)
                if !ok || mention.BotID != "" || mention.User == "" {
                    continue
                }

                go handleMention(api, mention)

            case socketmode.EventTypeSlashCommand:
                cmd, ok := evt.Data.(slack.SlashCommand)
                if !ok {
                    client.Ack(*evt.Request)
                    continue
                }

                if cmd.Command != "/goship" {
                    client.Ack(*evt.Request)
                    continue
                }

                var response string
                switch strings.ToLower(strings.TrimSpace(cmd.Text)) {
                case "hello":
                    response = "Hello from Go!"
                case "help":
                    response = "Try /goship or /goship hello."
                default:
                    response = "Go Ship! Your slash command works."
                }

                blocks := []slack.Block{
                    slack.NewHeaderBlock(
                        slack.NewTextBlockObject(slack.PlainTextType, "Go Ship!", false, false),
                    ),
                    slack.NewSectionBlock(
                        slack.NewTextBlockObject(
                            slack.MarkdownType,
                            "*Go Ship!* " + response,
                            false, false,
                        ),
                        nil, nil,
                    ),
                    slack.NewDividerBlock(),
                }

                if err := client.Ack(*evt.Request, map[string]any{
                    "response_type": "ephemeral",
                    "text":          response,
                    "blocks":        blocks,
                }); err != nil {
                    log.Println("Failed to respond to command:", err)
                }
            }
        }
    }()

    log.Println("Connecting to Slack...")
    if err := client.Run(); err != nil {
        log.Fatal(err)
    }
}

func handleMention(api *slack.Client, mention *slackevents.AppMentionEvent) {
    threadTS := mention.ThreadTimeStamp
    if threadTS == "" {
        threadTS = mention.TimeStamp
    }

    _, _, err := api.PostMessage(
        mention.Channel,
        slack.MsgOptionText("Hey <@"+mention.User+">! Go Ship!", false),
        slack.MsgOptionTS(threadTS),
    )
    if err != nil {
        log.Println("Failed to reply to mention:", err)
    }
}
```

Try mentioning the bot in a regular channel message, then in an existing thread. It should reply in the correct thread both times. You can also try `/goship`, `/goship hello`, and `/goship help` to see the Block Kit response.

If it doesn't work, double-check your tokens, permissions, and event subscription. If you change the app's scopes, don't forget to reinstall it. Also, if `/goship` times out, check that the command gets acknowledged promptly.

---

That's the basics! You now have a simple Slack bot written in Go that responds to mentions, handles slash commands, and sends Block Kit messages. Some things you could try next:

- Make `/goship` return a random project idea.
- Use `cmd.Text` to support more arguments and subcommands.
- Add an interactive button that updates the original message when clicked.
- Store some data in SQLite and have the bot retrieve it.

You can find more examples in the [slack-go documentation](https://pkg.go.dev/github.com/slack-go/slack), the [slack-go repository](https://github.com/slack-go/slack), and the [Slack API documentation](https://docs.slack.dev/).
