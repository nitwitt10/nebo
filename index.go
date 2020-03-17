package api

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

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
		w.Write([]byte("slack verification failed:"))
		return
	}

	if client == nil {
		client = simpleforce.NewClient(sfURL, simpleforce.DefaultClientID, simpleforce.DefaultAPIVersion)
	}
	if client == nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("salesforce client was not created successfully"))
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
		responseJSON, err := getRep(s.Text)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		w.Header().Set("Content-type", "application/json")
		w.Write([]byte(responseJSON))
		return

	default:
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
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

type accountInfo struct {
	Website string
	Manager string
}

func getRep(search string) (string, error) {
	q := "SELECT Website, CS_Manager__r.Name, Chargify_MRR__c FROM Account WHERE Type = 'Customer' AND Website LIKE '%" + search + "%'"
	result, err := client.Query(q)
	if err != nil {
		return "", err
	}
	accounts := []*accountInfo{}
	for _, record := range result.Records {
		manager := record["CS_Manager__r"]
		managerName := "unknown"
		if manager != nil {
			if mapName, ok := (manager.(map[string]interface{}))["Name"]; ok {
				managerName = fmt.Sprintf("%s", mapName)
			}
		}
		accounts = append(accounts, &accountInfo{
			Website: fmt.Sprintf("%s", record["Website"]),
			Manager: fmt.Sprintf("%s", managerName),
			MRR: fmt.Sprintf("%s", record["Chargify_MRR__c"],
		})
	}
	accounts = cleanAndSort(accounts)
	responseJSON := formatAccountInfos(accounts, search)
	return responseJSON, nil
}

// example formatting here: https://api.slack.com/reference/messaging/attachments
func formatAccountInfos(accountInfos []*accountInfo, search string) string {
	initialText := "Reps for search: " + search
	if len(accountInfos) == 0 {
		initialText = "No results for: " + search
	}
	result := `{
		"response_type": "in_channel",
		"text": "` + initialText + `",
		"attachments": [`
	for _, ai := range accountInfos {
		color := "3A23AD" // Searchspring purple
		if ai.Manager == "unknown" {
			color = "FF0000" // red
		}
		result += `{
			"color":"#` + color + `", 
			"text":"` + ai.Manager + ` - MRR: $` + ai.MRR + `",
			"author_name": "` + ai.Website + `"
		},`
	}
	result += `
		]
	 }
	 `
	return result
}

func cleanAndSort(accounts []*accountInfo) []*accountInfo {
	for _, account := range accounts {
		w := account.Website
		if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
			w = w[strings.Index(w, ":")+3:]
		}
		if strings.HasPrefix(w, "www.") {
			w = w[4:]
		}
		if strings.HasSuffix(w, "/") {
			w = w[0 : len(w)-1]
		}
		account.Website = w
	}
	sort.Slice(accounts, func(i, j int) bool {
		return len(accounts[i].Website) < len(accounts[j].Website)
	})
	return accounts
}
