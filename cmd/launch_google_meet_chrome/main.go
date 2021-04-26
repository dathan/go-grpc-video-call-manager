package main

import (
	"context"
	"fmt"
	"net"
	"os"

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

// launch the grpc server that handles the open of the chrome launch and actions
// launch a go routine that polls the calendar and schedules the next link
func main() {

	ctx := context.Background()

	go launchGRPCServer(ctx)

	t := meettask.GetTasks()

	// Convert the tasks into a cron task
	cron := tasks.NewCron(ctx, t)

	// refresh the task list
	go meettask.UpdateCronMeetings(ctx, cron)

	// do while
	go cron.Run()
	go cron.Loop()

	//TODO add signal catching
	<-ctx.Done()
	logrus.Println("SHUTDOWN")
	os.Exit(0)

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

	<-ctx.Done()
	logrus.Infoln("GRPC-Server is shutting down")
	s.GracefulStop()
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
