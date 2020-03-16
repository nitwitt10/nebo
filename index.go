package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/nlopes/slack"
	"github.com/simpleforce/simpleforce"
)

var client *simpleforce.Client

// Handler - check routing and call correct methods
func Handler(w http.ResponseWriter, r *http.Request) {
	slackVerificationCode, sfURL, sfUser, sfPassword, sfToken, err := getEnvironmentValues()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	s, err := slack.SlashCommandParse(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	if !s.ValidateToken(slackVerificationCode) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("slack verification failed"))
		return
	}

	if client == nil {
		client = simpleforce.NewClient(sfURL, simpleforce.DefaultClientID, simpleforce.DefaultAPIVersion)
	}
	if client == nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	err = client.LoginPassword(sfUser, sfPassword, sfToken)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	switch s.Command {
	case "/rep":
		rep, err := getRep(s.Text)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		w.Header().Set("Content-type", "application/json")
		w.Write([]byte(`{
			"response_type": "in_channel",
			"text": "Rep: ` + rep + `"
		}`))

	default:
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func getRep(url string) (string, error) {
	// TODO sanatize input to prevent accidental SQL injection from people in our slack.
	q := "SELECT Website, CS_Manager__r.Name FROM Account WHERE Website like '%" + url + "'"
	result, err := client.Query(q)
	if err != nil {
		return "", err
	}

	name := "unknown"
	for _, record := range result.Records {
		name = fmt.Sprintf("%s", record["Website"]) + ": " + fmt.Sprintf("%s", record["CS_Manager__r"])
	}
	return name, nil
}

func getEnvironmentValues() (string, string, string, string, string, error) {
	if os.Getenv("SLACK_VERIFICATION_TOKEN") == "" {
		return "", "", "", "", "", fmt.Errorf("Must set: SLACK_VERIFICATION_TOKEN")
	}
	if os.Getenv("SF_URL") == "" {
		return "", "", "", "", "", fmt.Errorf("Must set: SF_URL")
	}
	if os.Getenv("SF_USER") == "" {
		return "", "", "", "", "", fmt.Errorf("Must set: SF_USER")
	}
	if os.Getenv("SF_PASSWORD") == "" {
		return "", "", "", "", "", fmt.Errorf("Must set: SF_PASSWORD")
	}
	if os.Getenv("SF_TOKEN") == "" {
		return "", "", "", "", "", fmt.Errorf("Must set: SF_TOKEN")
	}
	return os.Getenv("SLACK_VERIFICATION_TOKEN"),
		os.Getenv("SF_URL"),
		os.Getenv("SF_USER"),
		os.Getenv("SF_PASSWORD"),
		os.Getenv("SF_TOKEN"), nil
}
