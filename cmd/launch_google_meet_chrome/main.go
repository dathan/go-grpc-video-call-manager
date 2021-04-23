package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/dathan/go-grpc-video-call-manager/pkg/calendar"
	"github.com/dathan/go-grpc-video-call-manager/pkg/manager"
	"github.com/dathan/go-grpc-video-call-manager/pkg/session"
	"github.com/dathan/go-grpc-video-call-manager/pkg/tasks"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// launch the grpc server that handles the open of the chrome server
// launch a go routine that polls the calendar and schedules the next link
//  - TODO - if a session is running inject a message to say do you want to start another session?
//  - TODO - when the time for the meeting is about to start, launch the meeting
//	- TODO - handle context
func main() {
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})

	logrus.SetReportCaller(true)
	logrus.SetLevel(logrus.InfoLevel)

	ctx := context.Background()

	go launchGRPCServer(ctx)

	for {
		// do while
		meetings, err := findMeetings()
		if err != nil {
			panic(err)
		}

		t, _ := taskWrapper(meetings)
		cron := tasks.NewCronTask(ctx, t)

		go timerMeeting(ctx, cron)

		cron.Run()

		//

		select {
		case <-ctx.Done():
			logrus.Infoln("Catch a shutdown")
			os.Exit(0)
		}
	}
}

// keep running a timer in a go-routine to look for new meetings
func timerMeeting(ctx context.Context, cron *tasks.Cron) {
	t := time.NewTimer(time.Duration(30) * time.Second)
	select {
	case <-ctx.Done():
		t.Stop()
		return

	case <-t.C:
		meetings, err := findMeetings()
		if err != nil {
			panic(err)
		}

		tsks, err := taskWrapper(meetings)
		if err != nil {
			panic(err) // should cause a signal
		}

		// go through all the tasks and skip the meetings that have started
		for i := len(tsks) - 1; i >= 0; i-- {
			if tsks[i].Start().Before(time.Now()) {
				tsks = removeElement(i, tsks)
			}
		}

		cron.Update(tsks)
		timerMeeting(ctx, cron) // keep going forever
	}

}

func removeElement(s int, tsks tasks.SequentialTasks) tasks.SequentialTasks {
	logrus.Infof("Removing %d ]  %v", s, tsks[s])
	tsks = append(tsks[:s], tsks[s+1:]...)
	return tsks
}

// findMeetings is invoked via a go-routine which periodically polls the calender to update the meetings for the day.
func findMeetings() (calendar.MeetItems, error) {

	logrus.Info("Looking for new meetings")

	cal := &calendar.CalService{}
	return cal.GetUpcomingMeetings()

}

type MeetTaskImpl struct {
	calendar.MeetItem
}
type MeetTasks []MeetTaskImpl

func taskWrapper(c calendar.MeetItems) (tasks.SequentialTasks, error) {
	var mt tasks.SequentialTasks
	for _, task := range c {
		mti := &MeetTaskImpl{task}
		mt = append(mt, mti)
	}
	return mt, nil

}

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

// setup the tasks to run on a timer.
func setTimers(ctx context.Context, c calendar.MeetItems) {

}

// launchGRPCServer is launched via a go routine
func launchGRPCServer(ctx context.Context) {
	port := 8080
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logrus.Fatalf("could not listen to port %d: %v", port, err)
	}

	logrus.Infof("GRPCServer starting localhost:%d\n", port)

	s := grpc.NewServer()
	manager.RegisterOpenMeetUrlServer(s, server{})
	err = s.Serve(lis)
	if err != nil {
		logrus.Fatalf("could not serve: %v", err)
	}

	select {
	case <-ctx.Done():
		logrus.Infoln("GRPC-Server is shutting down")
		s.GracefulStop()
		break

	}

}

type server struct {
	manager.UnimplementedOpenMeetUrlServer
}

// for the local server open the meet url
func (s server) OpenMeetUrl(c context.Context, man *manager.Meet) (*manager.Status, error) {

	meet, err := session.NewSession()
	if err != nil {
		return &manager.Status{
			Ok:       false,
			ErrorMsg: err.Error(),
		}, err
	}

	ctx, cancel := meet.NewContext()
	defer cancel()

	err = meet.Login(ctx)

	if err != nil {
		return &manager.Status{
			Ok:       false,
			ErrorMsg: err.Error(),
		}, err
	}

	err = meet.Open(ctx, man.Uri)

	if err != nil {
		return &manager.Status{
			Ok:       false,
			ErrorMsg: err.Error(),
		}, err
	}

	err = meet.ApplySettings(ctx)
	if err != nil {
		return &manager.Status{
			Ok:       false,
			ErrorMsg: err.Error(),
		}, err
	}

	meet.Wait(ctx)

	ret := &manager.Status{
		Ok: true,
	}

	return ret, nil
}
