package main

import (
	"context"
	"os"

	meettask "github.com/dathan/go-grpc-video-call-manager/internal/tasks"
	"github.com/dathan/go-grpc-video-call-manager/pkg/tasks"
	"github.com/sirupsen/logrus"
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

	// start the server
	go meettask.GRPCServer(ctx)

	t := meettask.GetTasks()

	// Convert the tasks into a cron task
	cron := tasks.NewCron(ctx, t)

	// refresh the task list
	go meettask.UpdateCronMeetings(ctx, cron)

	// do while
	go cron.Run()

	//TODO add signal catching
	<-ctx.Done()
	logrus.Println("SHUTDOWN")
	os.Exit(0)

}
