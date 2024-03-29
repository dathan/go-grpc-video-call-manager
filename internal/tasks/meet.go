package tasks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dathan/go-grpc-video-call-manager/internal/utils"
	"github.com/dathan/go-grpc-video-call-manager/pkg/calendar"
	"github.com/dathan/go-grpc-video-call-manager/pkg/manager"
	"github.com/dathan/go-grpc-video-call-manager/pkg/tasks"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// support for a magic number in seconds
const MEETING_FETCH_DELTA = 30

// 'extend' a meetitem
type MeetTaskImpl struct {
	calendar.MeetItem
}

// collection of meet taks
type MeetTasks []MeetTaskImpl

func (m *MeetTaskImpl) String() string {
	return fmt.Sprintf("Meeting Task: %s Starts: %s and Ends: %s", m.Name(), m.Start(), m.End())
}

// implement the Interface requried for Cron
func (m *MeetTaskImpl) Name() string {
	return m.Summary + " => [ " + m.Uri + " ]"
}

// return the start time
func (m *MeetTaskImpl) Start() time.Time {
	return m.StartTime
}

// return the end time
func (m *MeetTaskImpl) End() time.Time {
	return m.EndTime
}

// Run the current task from the cron package
func (m *MeetTaskImpl) Execute(config *utils.Config) error {

	if time.Since(m.Start()).Minutes() > 10.0 { // we start 10 minutes early if the meeting has started already skip
		logrus.Warnf("MEET TASK IS OLD NEED TO SKIP!! %s", m)
		return errors.New("Task is too old to run..skipping")
	}

	logrus.Infof("Execute !!!: %s => %s\n", m.Summary, m.Uri)
	backend := config.Backend

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
		logrus.Errorf("EXECUTE ERROR: %s", err.Error())
		return err
	}

	if stat.ErrorMsg != "" {
		logrus.Errorf("Server ERROR: %s", stat.ErrorMsg)
		return errors.New("GRPC SERVER ERROR: " + stat.ErrorMsg)
	}
	return nil
}

// keep running a timer in a go-routine to look for new meetings
func UpdateCronMeetings(ctx context.Context, cron *tasks.Cron) {
	// note cannot use a ticker, since that keeps fireing even if that timer body is blocked?
	t := time.NewTimer(time.Duration(MEETING_FETCH_DELTA) * time.Second)
	defer func() {
		if !t.Stop() {
			<-t.C
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case <-t.C:

			tsks, err := GetTasks(cron.Config)
			if err != nil {
				// When I close my laptop for the day, I want the client to recover
				for { //TODO: 1140 minutes in a day, wait a day -- fix this
					logrus.Errorf("Error from find meetings - retrying: %s", err)
					// send a message to reset the channels (laptop is sleep)
					cron.Update(tasks.SequentialTasks{})
					time.Sleep(1 * time.Minute)
					tsks, err = GetTasks(cron.Config)
					if err == nil {
						break
					}
				}
			}

			// go through all the tasks and skip the meetings that have started
			// tsks = PruneTasks(tsks)
			cron.Update(tsks) // this will block and we want that if things are running so a dogpile of events do not happen
			UpdateCronMeetings(ctx, cron)
			return
		}
	}
}

// TODO: think of how to do this more efficiently without copies just to know
func PruneTasks(tsks tasks.SequentialTasks) tasks.SequentialTasks {
	for i := len(tsks) - 1; i >= 0; i-- {
		// note this should look at the status of a job running
		t := time.Now().Add(-MEETING_FETCH_DELTA * time.Second)
		if tsks[i].Start().Before(t) { // if now() == 10 am, if task.Start == 10 am is before 9:50
			logrus.Infof("Removing %d ]  %s", i, tsks[i])
			tsks = append(tsks[:i], tsks[i+1:]...)
		}
	}

	return tsks
}

// common code to getTasks
func GetTasks(c *utils.Config) (tasks.SequentialTasks, error) {

	meetings, err := FindMeetings(c)
	if err != nil {
		return nil, err
	}

	return TaskWrapper(meetings), nil

}

// findMeetings is invoked via a go-routine which periodically polls the calender to update the meetings for the day.
func FindMeetings(c *utils.Config) (calendar.MeetItems, error) {

	cal := calendar.NewCalService(c)
	return cal.GetUpcomingMeetings()

}

// convert the meeting items to meeting tasks
func TaskWrapper(c calendar.MeetItems) tasks.SequentialTasks {
	var mt tasks.SequentialTasks
	for _, task := range c {
		mti := &MeetTaskImpl{task}
		mt = append(mt, mti)
	}
	return mt

}
