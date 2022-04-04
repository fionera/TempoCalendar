package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/arran4/golang-ical"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"time"
)

type Config struct {
	Username string `toml:"username"`
	Password string `toml:"password"`
	JiraURL  string `toml:"url"`
}

var re = regexp.MustCompile(`(?m)name="ajs-tempo-user-key" content="([\w\.]+)"`)

func main() {
	var config Config
	_, err := toml.DecodeFile("config.toml", &config)
	if err != nil {
		log.Fatal(err)
	}

	jar, _ := cookiejar.New(nil)
	c := http.Client{Jar: jar}

	log.Println("Logging in")
	resp, err := c.PostForm(config.JiraURL+"/login.jsp", url.Values{
		"os_username":    {config.Username},
		"os_password":    {config.Password},
		"os_destination": {"/secure/Tempo.jspa"},
		"user_role":      {},
		"atl_token":      {},
		"login":          {"Anmelden"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var userID string
	for _, match := range re.FindAllStringSubmatch(string(data), -1) {
		log.Println("Found UserID:", match[1])
		userID = match[1]
		break
	}

	if userID == "" {
		log.Println("Empty UserID. Exiting")
		return
	}

	handler := func(w http.ResponseWriter, req *http.Request) {
		log.Println("Fetching...")

		b, err := json.Marshal(Request{
			From:  time.Now().AddDate(0, -1, 0).Format("2006-01-02"),
			To:    time.Now().AddDate(0, 3, 0).Format("2006-01-02"),
			Users: []string{userID},
		})
		if err != nil {
			panic(err)
		}

		resp, err = c.Post(config.JiraURL+"/rest/tempo-planning/2/resource-planning/search", "application/json", bytes.NewReader(b))
		if err != nil {
			log.Fatal(err)
		}

		var r Response
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			panic(err)
		}

		cal := ics.NewCalendar()
		cal.SetMethod(ics.MethodRequest)

		for _, r := range r.UserResources {
			for _, p := range r.Planlogs {
				title := p.PlanItemInfo.Summary
				if title == "" {
					title = p.PlanItemInfo.Name
				}

				start, err := time.Parse("2006-01-02", p.PlanStart)
				if err != nil {
					panic(err)
				}
				start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.Now().Location())

				end, err := time.Parse("2006-01-02", p.PlanEnd)
				if err != nil {
					panic(err)
				}
				end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 0, 0, time.Now().Location())

				event := cal.AddEvent(fmt.Sprintf("%d@jira.babiel.com", p.AllocationId))
				event.SetCreatedTime(time.Now())
				event.SetDtStampTime(time.Now())
				event.SetModifiedAt(time.Now())
				event.SetStartAt(start)
				event.SetEndAt(end)
				event.SetSummary(title)
			}
		}

		w.Write([]byte(cal.Serialize()))
	}

	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}

type Request struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Users []string `json:"users"`
}

type Response struct {
	UserResources map[string]struct {
		User     string `json:"user"`
		Planlogs []struct {
			AllocationId int    `json:"allocationId"`
			Assignee     string `json:"assignee"`
			AssigneeType string `json:"assigneeType"`
			PlanItemId   int    `json:"planItemId"`
			PlanItemType string `json:"planItemType"`
			PlanItemInfo struct {
				Key          string        `json:"key"`
				Id           int           `json:"id"`
				Type         string        `json:"type"`
				Name         string        `json:"name"`
				Summary      string        `json:"summary,omitempty"`
				IconName     string        `json:"iconName,omitempty"`
				IconUrl      string        `json:"iconUrl,omitempty"`
				ProjectKey   string        `json:"projectKey"`
				ProjectId    int           `json:"projectId"`
				Components   []interface{} `json:"components,omitempty"`
				Versions     []interface{} `json:"versions,omitempty"`
				ProjectColor string        `json:"projectColor"`
				AvatarUrls   struct {
					X48 string `json:"48x48"`
					X24 string `json:"24x24"`
					X16 string `json:"16x16"`
					X32 string `json:"32x32"`
				} `json:"avatarUrls"`
				PlanItemUrl string `json:"planItemUrl"`
				AccountKey  string `json:"accountKey,omitempty"`
				IsResolved  bool   `json:"isResolved,omitempty"`
				IssueStatus struct {
					Name  string `json:"name"`
					Color string `json:"color"`
				} `json:"issueStatus,omitempty"`
				Description               string `json:"description,omitempty"`
				EstimatedRemainingSeconds int    `json:"estimatedRemainingSeconds,omitempty"`
				OriginalEstimateSeconds   int    `json:"originalEstimateSeconds,omitempty"`
			} `json:"planItemInfo"`
			PlanDescription       string  `json:"planDescription"`
			Day                   string  `json:"day"`
			TimePlannedSeconds    float64 `json:"timePlannedSeconds"`
			SecondsPerDay         int     `json:"secondsPerDay"`
			IncludeNonWorkingDays bool    `json:"includeNonWorkingDays"`
			PlanCreator           string  `json:"planCreator"`
			DateCreated           string  `json:"dateCreated"`
			DateUpdated           string  `json:"dateUpdated"`
			PlanStart             string  `json:"planStart"`
			PlanStartTime         string  `json:"planStartTime"`
			PlanEnd               string  `json:"planEnd"`
			Location              struct {
				Name string `json:"name"`
				Id   int    `json:"id"`
			} `json:"location"`
			PlanApproval struct {
				Requester struct {
					Name        string `json:"name"`
					Key         string `json:"key"`
					DisplayName string `json:"displayName"`
					Avatar      string `json:"avatar"`
				} `json:"requester"`
				Reviewer struct {
					Name        string `json:"name"`
					Key         string `json:"key"`
					DisplayName string `json:"displayName"`
					Avatar      string `json:"avatar"`
				} `json:"reviewer"`
				Actor struct {
					Name        string `json:"name"`
					Key         string `json:"key"`
					DisplayName string `json:"displayName"`
					Avatar      string `json:"avatar"`
				} `json:"actor"`
				StatusCode   int `json:"statusCode"`
				LatestAction struct {
					Key     string `json:"key"`
					Message string `json:"message"`
				} `json:"latestAction"`
				Updated string `json:"updated"`
				Created string `json:"created"`
			} `json:"planApproval,omitempty"`
		} `json:"planlogs"`
		Workload map[string]struct {
			Date            string `json:"date"`
			RequiredSeconds int    `json:"requiredSeconds"`
			Type            string `json:"type"`
			Holiday         struct {
				Name            string `json:"name"`
				Description     string `json:"description"`
				DurationSeconds int    `json:"durationSeconds"`
			} `json:"holiday"`
		} `json:"workload"`
	} `json:"userResources"`
	GenericResources struct {
	} `json:"genericResources"`
}
