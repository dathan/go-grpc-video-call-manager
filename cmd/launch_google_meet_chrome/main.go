package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	meettask "github.com/dathan/go-grpc-video-call-manager/internal/tasks"
	"github.com/dathan/go-grpc-video-call-manager/pkg/tasks"
	"github.com/sirupsen/logrus"
)

//setup logging
func init() {
	l := &logrus.TextFormatter{ForceColors: true, FullTimestamp: true, TimestampFormat: "2006-01-02 15:04:05"}
	logrus.SetFormatter(l)
	logrus.SetReportCaller(true)
	logrus.SetLevel(logrus.InfoLevel)
}

// launch the grpc server that handles the open of the chrome launch and actions
// launch a go routine that polls the calendar and schedules the next link
func main() {

	ctx := context.Background()

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// start the server
	go meettask.GRPCServer(ctx)

	// get tasks that implement the interface
	t := meettask.GetTasks()

	// Convert the tasks into a cron task
	cron := tasks.NewCron(ctx, t)

	// refresh the task list
	go meettask.UpdateCronMeetings(ctx, cron)

	// do while
	go cron.Run()

	<-ctx.Done()
	logrus.Println("SHUTDOWN")
	os.Exit(0)
}
