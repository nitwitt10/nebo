package api

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
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
	case "/rep", "/nebo-alpha":
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
		w.Write([]byte("unknown slash command " + s.Command))
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
		os.Getenv("SF_TOKEN"),
		nil
}

type accountInfo struct {
	Website   string
	Manager   string
	Active    string
	MRR       float64
	FamilyMRR float64
	Platform  string
}

func getRep(search string) (string, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9_.-]+")
	if err != nil {
		return "", err
	}

	sanitized := reg.ReplaceAllString(search, "")

	q := "SELECT Type, Website, CS_Manager__r.Name, Family_MRR__c, Chargify_MRR__c, Platform__c FROM Account WHERE Type IN ('Customer', 'Inactive Customer') AND Website LIKE '%" + sanitized + "%'"
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
		Type := record["Type"]
		active := "Yes"
		if Type != "Customer" {
			active = "Not active"
		}
		platform := "unknown"
		if record["Platform__c"] != nil {
			platform = fmt.Sprintf("%s", record["Platform__c"])
		}
		mrr := float64(-1)
		if record["Chargify_MRR__c"] != nil {
			mrr = record["Chargify_MRR__c"].(float64)
		}
		familymrr := float64(-1)
		if record["Family_MRR__c"] != nil {
			familymrr = record["Family_MRR__c"].(float64)
		}

		accounts = append(accounts, &accountInfo{
			Website:   fmt.Sprintf("%s", record["Website"]),
			Manager:   fmt.Sprintf("%s", managerName),
			Active:    fmt.Sprintf("%s", active),
			MRR:       mrr,
			FamilyMRR: familymrr,
			Platform:  platform,
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
		mrr := "unknown"
		if ai.MRR != -1 {
			mrr = fmt.Sprintf("$%.2f", ai.MRR)
		}
		familymrr := "unknown"
		if ai.FamilyMRR != -1 {
			familymrr = fmt.Sprintf("$%.2f", ai.FamilyMRR)
		}
		result += `{
			"color":"#` + color + `", 
			"text":"Rep: ` + ai.Manager + `\n MRR: ` + mrr + `\n Family MRR: ` + familymrr + `\n Platform: ` + ai.Platform + `\n Active: ` + ai.Active + `",
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
