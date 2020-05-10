package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/nlopes/slack"
	"github.com/simpleforce/simpleforce"
)

var platforms = []string{"Yahoo",
	"CommerceV3",
	"Magento",
	"3dcart",
	"Netsuite",
	"BigCommerce",
	"Other",
	"Miva",
	"Custom",
	"Shopify"}

type SalesForceDAO interface {
	Query(query string) (*simpleforce.QueryResult, error)
}

type SalesForceDAOImpl struct {
	Client *simpleforce.Client
}

func (s *SalesForceDAOImpl) Query(query string) (*simpleforce.QueryResult, error) {
	return s.Client.Query(query)
}

var salesForceDAO SalesForceDAO = nil

func NewSalesForceDAO(sfURL string, sfUser string, sfPassword string, sfToken string) (SalesForceDAO, error) {
	client := simpleforce.NewClient(sfURL, simpleforce.DefaultClientID, simpleforce.DefaultAPIVersion)
	if client == nil {
		return nil, fmt.Errorf("nil returned from client creation")
	}
	err := client.LoginPassword(sfUser, sfPassword, sfToken)
	if err != nil {
		return nil, err
	}
	return &SalesForceDAOImpl{
		Client: client,
	}, nil
}

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

	if salesForceDAO == nil {
		salesForceDAO, err = NewSalesForceDAO(sfURL, sfUser, sfPassword, sfToken)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("salesforce client was not created successfully: " + err.Error()))
			return
		}
	}

	switch s.Command {
	case "/rep", "/alpha-nebo", "/nebo":
		responseJSON, err := getResponse(salesForceDAO, s.Text)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		w.Header().Set("Content-type", "application/json")
		w.Write(responseJSON)
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

func getResponse(sfDAO SalesForceDAO, search string) ([]byte, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9_.-]+")
	if err != nil {
		return nil, err
	}

	sanitized := reg.ReplaceAllString(search, "")

	q := "SELECT Type, Website, CS_Manager__r.Name, Family_MRR__c, Chargify_MRR__c, Platform__c " +
		"FROM Account WHERE Type IN ('Customer', 'Inactive Customer') " +
		"AND (Website LIKE '%" + sanitized + "%' OR Platform__c LIKE '%" + sanitized + "%') ORDER BY Chargify_MRR__c DESC"
	result, err := sfDAO.Query(q)

	if err != nil {
		return nil, err
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
		fmt.Println(fmt.Sprintf("%s", record["Website"]), record["Chargify_MRR__c"])
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
	accounts = cleanAccounts(accounts)
	if !isPlatformSearch(search) {
		accounts = sortAccounts(accounts)
	}
	accounts = truncateAccounts(accounts)
	msg := formatAccountInfos(accounts, search)
	return json.Marshal(msg)
}

func truncateAccounts(accounts []*accountInfo) []*accountInfo {
	truncated := []*accountInfo{}
	for i, account := range accounts {
		if i == 20 {
			break
		}
		truncated = append(truncated, account)
	}
	return truncated
}
func isPlatformSearch(search string) bool {
	for _, platform := range platforms {
		if strings.ToLower(search) == strings.ToLower(platform) {
			return true
		}
	}
	return false
}

// example formatting here: https://api.slack.com/reference/messaging/attachments
func formatAccountInfos(accountInfos []*accountInfo, search string) *slack.Msg {
	initialText := "Reps for search: " + search
	if len(accountInfos) == 0 {
		initialText = "No results for: " + search
	}

	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeInChannel,
		Text:         initialText,
		Attachments:  []slack.Attachment{},
	}
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
		mrr = mrr + " (Family MRR: " + familymrr + ")"
		text := "Rep: " + ai.Manager + "\nMRR: " + mrr + "\nPlatform: " + ai.Platform + "\nActive: " + ai.Active
		msg.Attachments = append(msg.Attachments, slack.Attachment{
			Color:      "#" + color,
			Text:       text,
			AuthorName: ai.Website,
		})
	}
	return msg
}

func cleanAccounts(accounts []*accountInfo) []*accountInfo {
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
	return accounts
}
func sortAccounts(accounts []*accountInfo) []*accountInfo {
	sort.Slice(accounts, func(i, j int) bool {
		return len(accounts[i].Website) < len(accounts[j].Website)
	})
	return accounts
}
