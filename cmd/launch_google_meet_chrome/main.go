package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	meettask "github.com/dathan/go-grpc-video-call-manager/internal/tasks"
	"github.com/dathan/go-grpc-video-call-manager/pkg/manager"
	"github.com/dathan/go-grpc-video-call-manager/pkg/session"
	"github.com/dathan/go-grpc-video-call-manager/pkg/tasks"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

//setup logging
func init() {
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	logrus.SetReportCaller(true)
	logrus.SetLevel(logrus.InfoLevel)
}

// launch the grpc server that handles the open of the chrome server
// launch a go routine that polls the calendar and schedules the next link
func main() {

	ctx := context.Background()

	go launchGRPCServer(ctx)

	for {

		t := meettask.GetTasks()
		cron := tasks.NewCronTask(ctx, t)

		go timerMeeting(ctx, cron)

		cron.Run()

		//when an interupt to stop is caught like a control-c
		select {
		case <-ctx.Done():
			logrus.Println("SHUTDOWN")
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
		tsks := meettask.GetTasks()
		// go through all the tasks and skip the meetings that have started
		tsks = meettask.PruneTasks(tsks)
		cron.Update(tsks)
		timerMeeting(ctx, cron) // keep going forever
	}

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
