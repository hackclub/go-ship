package slack

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

var api *slack.Client
var isInited bool = false

func EventsEndpoint(w http.ResponseWriter, r *http.Request) {
	if !isInited {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Slack bot not initialized"))
		return
	}
	signingSecret := os.Getenv("SLACK_SIGNING_SECRET")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sv, err := slack.NewSecretsVerifier(r.Header, signingSecret)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := sv.Write(body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := sv.Ensure(); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Println("[INFO] Received event:", eventsAPIEvent.Type)
	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(body), &r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(r.Challenge))
	case slackevents.CallbackEvent:
		innerEvent := eventsAPIEvent.InnerEvent
		fmt.Println("[INFO] Received inner event:", innerEvent.Type)
		// THROW EVENTS HERE U CANT MISS THIS COMMENT
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MemberJoinedChannelEvent:
			if ev.Channel != os.Getenv("SLACK_CHANNEL_ID") {
				return
			}

			// Fetch user info to check if they are a bot
			user, err := api.GetUserInfo(ev.User)
			if err != nil {
				fmt.Printf("Error fetching user info: %v\n", err)
				return
			}

			// Don't send welcome message to bots
			if user.IsBot {
				return
			}

			// send welcome message
			_, _, err = api.PostMessage(
				ev.Channel,
				slack.MsgOptionText(fmt.Sprintf("Welcome <@%s>! :party-gopher:", ev.User), false),
			)
			if err != nil {
				fmt.Printf("Error sending welcome message: %v\n", err)
			}
		case *slackevents.AppMentionEvent:
			opts := []slack.MsgOption{
				slack.MsgOptionText(":super-party-gopher:", false),
			}
			if ev.ThreadTimeStamp != "" {
				opts = append(opts, slack.MsgOptionTS(ev.ThreadTimeStamp))
			}

			_, _, err = api.PostMessage(
				ev.Channel,
				opts...,
			)
			if err != nil {
				fmt.Printf("Error sending mention response: %v\n", err)
			}
		default:
			fmt.Printf("[INFO] Unhandled inner event type: %T\n", ev)
		}
	}
}

func Init() {
	token := os.Getenv("SLACK_BOT_TOKEN")
	if token == "" {
		fmt.Println("No slack bot token, bot functionality will be disabled.")
		isInited = true
		return
	}
	api = slack.New(token)

	// test
	var err error
	for i := range 3 {
		_, err = api.AuthTest()
		if err == nil {
			isInited = true
			return
		}
		fmt.Printf("Failed to authenticate with Slack API (try %d): %v\n", i+1, err)
		time.Sleep(time.Second * 2)
	}
	if err != nil {
		panic(fmt.Sprintf("Failed to authenticate with Slack API after 3 attempts: %v", err))
	}
}

func AddPersonToChannel(userId string) error {
	if !isInited { return nil }
	channelId := os.Getenv("SLACK_CHANNEL_ID")
	if channelId == "" {
		return fmt.Errorf("SLACK_CHANNEL_ID environment variable not set")
	}

	_, err := api.InviteUsersToConversation(channelId, userId)
	if err != nil {
		return fmt.Errorf("failed to invite user to channel: %v", err)
	}
	return nil
}
