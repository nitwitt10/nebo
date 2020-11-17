package productboard

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/nlopes/slack"
	"github.com/searchspring/nebo/validator"
)

// DAO acts as the productboard DAO
type DAO interface {
	CreateNote(query string) ([]byte, error)
}

// DAOImpl defines the properties of the DAO
type DAOImpl struct {
	Client    *http.Client
	// User      string
	// Password  string
	// Customers map[string][]string
}

// NewDAO returns the productboard DAO
func NewDAO() DAO {
	if validator.ContainsEmptyString(nxUser, nxPassword) {
		return nil
	}
	return &DAOImpl{
		// User:     nxUser,
		// Password: nxPassword,
		Client:   http.DefaultClient,
	}
}

// <Example JSON response here>
type resultData struct {
	Data [][]string `json:"data"`
}

// <DESCRIPTION>
func (d *DAOImpl) CreateNote(content string) ([]byte, error) {
	// if d.Customers == nil {
	// 	res, err := d.Client.Get("http://" + d.User + ":" + d.Password + "@client-report.nxtpd.com/api/data-table.php?table=accounts&_=1592606239141")
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	body, err := ioutil.ReadAll(res.Body)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	resultData := &resultData{}

	// 	err = json.Unmarshal(body, resultData)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	d.Customers = map[string][]string{}
	// 	for _, row := range resultData.Data {
	// 		d.Customers[row[0]] = row
	// 	}
	// }
	// msg := d.findMatch(query)
	// return json.Marshal(msg)



	return featureResponse(content)
}

func featureResponse(content string) []byte {
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeEphemeral,
		Text:         "Feature request submitted! The Product team will be in touch.",
	}
	json, _ := json.Marshal(msg)
	return json
}

