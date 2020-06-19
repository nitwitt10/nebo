package api

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/nlopes/slack"
	"searchspring.com/slack/salesforce"
)

var salesForceDAO salesforce.DAO = nil

// Handler - check routing and call correct methods
func Handler(res http.ResponseWriter, req *http.Request) {
	slackVerificationCode, slackOauthToken, sfURL, sfUser, sfPassword, sfToken, err := getEnvironmentValues()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(err.Error()))
		return
	}

	s, err := slack.SlashCommandParse(req)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(err.Error()))
		return
	}

	if !s.ValidateToken(slackVerificationCode) {
		res.WriteHeader(http.StatusUnauthorized)
		res.Write([]byte("slack verification failed:"))
		return
	}

	if salesForceDAO == nil {
		salesForceDAO, err = salesforce.NewDAO(sfURL, sfUser, sfPassword, sfToken)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte("salesforce client was not created successfully: " + err.Error()))
			return
		}
	}

	switch s.Command {
	case "/rep", "/alpha-nebo", "/nebo":
		if strings.TrimSpace(s.Text) == "help" || strings.TrimSpace(s.Text) == "" {
			writeHelpNebo(res)
			return
		}
		responseJSON, err := salesForceDAO.Query(s.Text)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte(err.Error()))
			return
		}
		res.Header().Set("Content-type", "application/json")
		res.Write(responseJSON)
		return

	case "/feature":
		if strings.TrimSpace(s.Text) == "help" || strings.TrimSpace(s.Text) == "" {
			writeHelpFeature(res)
			return
		}
		responseJSON := featureResponse(s.Text)
		sendSlackMessage(slackOauthToken, s.Text, s.UserID)
		res.Header().Set("Content-type", "application/json")
		res.Write(responseJSON)
		return

	case "/meet":
		if strings.TrimSpace(s.Text) == "help" {
			writeHelpMeet(res)
			return
		}
		responseJSON := meetResponse(s.Text)
		res.Header().Set("Content-type", "application/json")
		res.Write(responseJSON)
		return
	default:
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte("unknown slash command " + s.Command))
		return
	}
}

func writeHelpFeature(res http.ResponseWriter) {
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeEphemeral,
		Text:         "Feature usage:\n`/feature description of feature required` - submits a feature to the product team\n`/feature help` - this message",
	}
	json, _ := json.Marshal(msg)
	res.Header().Set("Content-type", "application/json")
	res.Write(json)
}

func writeHelpNebo(res http.ResponseWriter) {
	platformsJoined := strings.ToLower(strings.Join(salesforce.Platforms, ", "))
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeEphemeral,
		Text:         "Nebo usage:\n`/nebo shoes` - find all customers with shoe in the name\n`/nebo shopify` - show {" + platformsJoined + "} clients sorted by MRR\n`/nebo help` - this message",
	}
	json, _ := json.Marshal(msg)
	res.Header().Set("Content-type", "application/json")
	res.Write(json)
}

func writeHelpMeet(res http.ResponseWriter) {
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeEphemeral,
		Text:         "Meet usage:\n`/meet` - generate a random meet\n`/meet name` - generate a meet with a name\n`/meet help` - this message",
	}
	json, _ := json.Marshal(msg)
	res.Header().Set("Content-type", "application/json")
	res.Write(json)
}

func sendSlackMessage(token string, text string, authorID string) {
	api := slack.New(token)
	channelID, timestamp, err := api.PostMessage("G013YLWL3EX", slack.MsgOptionText("<@"+authorID+"> requests: "+text, false))
	if err != nil {
		fmt.Printf("%s\n", err)
		return
	}
	fmt.Printf("Message successfully sent to channel %s at %s", channelID, timestamp)
}

func featureResponse(search string) []byte {
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeEphemeral,
		Text:         "feature request submitted, we'll be in touch!",
	}
	json, _ := json.Marshal(msg)
	return json
}

func meetResponse(search string) []byte {
	name := search
	name = strings.ReplaceAll(name, " ", "-")
	if strings.TrimSpace(search) == "" {
		rand.Seed(time.Now().UnixNano())
		name = petname.Generate(3, "-")
	}
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeInChannel,
		Text:         "g.co/meet/" + name,
	}
	json, _ := json.Marshal(msg)
	return json
}

func getEnvironmentValues() (string, string, string, string, string, string, error) {
	if os.Getenv("SLACK_VERIFICATION_TOKEN") == "" {
		return "", "", "", "", "", "", fmt.Errorf("Must set: SLACK_VERIFICATION_TOKEN")
	}
	if os.Getenv("SLACK_OAUTH_TOKEN") == "" {
		return "", "", "", "", "", "", fmt.Errorf("Must set: SLACK_OAUTH_TOKEN")
	}
	if os.Getenv("SF_URL") == "" {
		return "", "", "", "", "", "", fmt.Errorf("Must set: SF_URL")
	}
	if os.Getenv("SF_USER") == "" {
		return "", "", "", "", "", "", fmt.Errorf("Must set: SF_USER")
	}
	if os.Getenv("SF_PASSWORD") == "" {
		return "", "", "", "", "", "", fmt.Errorf("Must set: SF_PASSWORD")
	}
	if os.Getenv("SF_TOKEN") == "" {
		return "", "", "", "", "", "", fmt.Errorf("Must set: SF_TOKEN")
	}
	return os.Getenv("SLACK_VERIFICATION_TOKEN"),
		os.Getenv("SLACK_OAUTH_TOKEN"),
		os.Getenv("SF_URL"),
		os.Getenv("SF_USER"),
		os.Getenv("SF_PASSWORD"),
		os.Getenv("SF_TOKEN"),
		nil
}
