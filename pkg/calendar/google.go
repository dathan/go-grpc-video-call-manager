package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

var TOKEN *oauth2.Token

// service to get the urls
type CalService struct {
	callersEmail string
}

// implement a task
type Task interface {
	When() time.Time
	End() time.Time
	Execute() error //A Task has to be able to be run
}

// MeetItem is a structure comtaining the pertinate meeting info
type MeetItem struct {
	Uri       string
	Summary   string
	StartTime time.Time
	EndTime   time.Time
}

// Collection
type MeetItems []MeetItem

//GetUpcomingMeetings returns a list of meetings to join for the day
func (em *CalService) GetUpcomingMeetings() (MeetItems, error) {
	meetings := MeetItems{}

	pwd, _ := os.Getwd() //TODO
	b, err := ioutil.ReadFile(pwd + "/cmd/launch_google_meet_chrome/credentials.json")
	if err != nil {
		log.Errorf("Unable to read client secret file: %v", err)
		return nil, err
	}

	// If modifying these scopes, delete your previously saved token.json.
	scopes := []string{calendar.CalendarScope, "email"}
	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		log.Errorf("Unable to parse client secret file to config: %v", err)
		return nil, err
	}

	client := getClient(config)

	em.callersEmail = "dathan.pattishall@wework.com"

	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Errorf("Unable to retrieve Calendar client: %v", err)
		return nil, err
	}

	t := time.Now().Format(time.RFC3339)
	events, err := srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		log.Errorf("Unable to retrieve next ten of the user's events: %v", err)
		return nil, err
	}

	if len(events.Items) == 0 {
		return meetings, errors.New("No Upcoming events")
	}

	for _, item := range events.Items {

		if item.ConferenceData != nil && item.ConferenceData.ConferenceSolution != nil && item.ConferenceData.ConferenceSolution.Name == "Google Meet" {
			for _, entry := range item.ConferenceData.EntryPoints {
				if entry.Uri != "" && entry.EntryPointType == "video" && (item.Organizer.Email == em.callersEmail || em.checkGoogleEventAttendies(item.Attendees)) {

					date := item.Start.DateTime
					if date == "" {
						date = item.Start.Date
					}

					st, _ := time.Parse(time.RFC3339, date)

					date = item.Start.DateTime
					if date == "" {
						date = item.Start.Date
					}

					et, _ := time.Parse(time.RFC3339, date)
					mi := MeetItem{
						Uri:       entry.Uri,
						StartTime: st,
						EndTime:   et,
						Summary:   item.Summary,
					}

					meetings = append(meetings, mi)
				}
			}
		}

	}

	return meetings, nil
}

// checks to see if the attendee is self
func (cs *CalService) checkGoogleEventAttendies(attendies []*calendar.EventAttendee) bool {

	for _, attendee := range attendies {
		//TODO get this from a config of who the caller is
		if attendee.Email == cs.callersEmail && attendee.ResponseStatus != "declined" {
			return true
		}
	}
	return false
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	TOKEN = tok
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Errorf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Errorf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Errorf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(token); err != nil {
		panic(err)
	}
}

func path() (string, error) {
	ex, err := os.Executable()
	if err == nil {
		return filepath.Dir(ex), nil
	}
	return "", err
}
