package tasks

import (
	"context"
	"errors"
	"time"

	"github.com/dathan/go-grpc-video-call-manager/pkg/calendar"
	"github.com/dathan/go-grpc-video-call-manager/pkg/manager"
	"github.com/dathan/go-grpc-video-call-manager/pkg/tasks"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type MeetTaskImpl struct {
	calendar.MeetItem
}

type MeetTasks []MeetTaskImpl

func (m *MeetTaskImpl) Start() time.Time {
	return m.StartTime
}

func (m *MeetTaskImpl) End() time.Time {
	return m.EndTime
}

func (m *MeetTaskImpl) Execute() error {
	logrus.Infof("Execute !!!: %s => %s\n", m.Summary, m.Uri)
	backend := "localhost:8080"

	conn, err := grpc.Dial(backend, grpc.WithInsecure())
	if err != nil {
		logrus.Fatalf("could not connect to %s: %v", backend, err)
	}
	defer conn.Close()

	client := manager.NewOpenMeetUrlClient(conn)
	meet := &manager.Meet{
		Uri:  m.Uri,
		Done: false,
	}

	stat, err := client.OpenMeetUrl(context.Background(), meet)
	if err != nil {
		logrus.Error("EXECUTE ERROR: %s\n", err.Error())
		return err
	}

	if stat.ErrorMsg != "" {
		logrus.Error("Server ERROR: %s\n", stat.ErrorMsg)
		return errors.New("GRPC SERVER ERROR: " + stat.ErrorMsg)
	}
	return nil
}

// TODO: think of how to do this more efficiently without copies just to know
func PruneTasks(tsks tasks.SequentialTasks) tasks.SequentialTasks {
	for i := len(tsks) - 1; i >= 0; i-- {
		if tsks[i].Start().Before(time.Now()) {
			tsks = RemoveElement(i, tsks)
		}
	}
	return tsks
}

// common code to getTasks
func GetTasks() tasks.SequentialTasks {
	meetings, err := FindMeetings()
	if err != nil {
		panic(err)
	}

	tsks, err := TaskWrapper(meetings)
	if err != nil {
		panic(err) // should cause a signal
	}
	return tsks
}

// todo can this be done without a copy? verify a copy.
func RemoveElement(s int, tsks tasks.SequentialTasks) tasks.SequentialTasks {
	logrus.Infof("Removing %d ]  %v", s, tsks[s])
	tsks = append(tsks[:s], tsks[s+1:]...)
	return tsks
}

// findMeetings is invoked via a go-routine which periodically polls the calender to update the meetings for the day.
func FindMeetings() (calendar.MeetItems, error) {

	logrus.Info("Looking for new meetings")

	cal := &calendar.CalService{}
	return cal.GetUpcomingMeetings()

}

func TaskWrapper(c calendar.MeetItems) (tasks.SequentialTasks, error) {
	var mt tasks.SequentialTasks
	for _, task := range c {
		mti := &MeetTaskImpl{task}
		mt = append(mt, mti)
	}
	return mt, nil

}
