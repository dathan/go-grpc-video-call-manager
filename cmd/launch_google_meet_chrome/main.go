package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	meettask "github.com/dathan/go-grpc-video-call-manager/internal/tasks"
	"github.com/dathan/go-grpc-video-call-manager/internal/utils"
	"github.com/dathan/go-grpc-video-call-manager/pkg/tasks"
	"github.com/sirupsen/logrus"
)

// setup logging
func init() {
	l := &logrus.TextFormatter{ForceColors: true, FullTimestamp: true, TimestampFormat: "2006-01-02 15:04:05"}
	logrus.SetFormatter(l)
	logrus.SetReportCaller(true)
	logrus.SetLevel(logrus.InfoLevel)
}

// launch the grpc server that handles the open of the chrome launch and actions
// launch a go routine that polls the calendar and schedules the next link
func main() {
	//trace.Start(os.Stderr)
	ctx := context.Background()

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	credentialsPaths := []string{
		filepath.Join(pwd, "credentials.json"),                                           // current directory
		filepath.Join(pwd, "..", "conf", "credentials.json"),                             // conf directory
		filepath.Join(pwd, "conf", "credentials.json"),                                   // conf directory
		filepath.Join(pwd, "..", "cmd", "launch_google_meet_chrome", "credentials.json"), // ../cmd/launch_google_meet_chrome directory
	}

	configPaths := []string{
		filepath.Join(pwd, "config.json"),                                           // current directory
		filepath.Join(pwd, "..", "conf", "config.json"),                             // conf directory
		filepath.Join(pwd, "conf", "config.json"),                                   // conf directory
		filepath.Join(pwd, "..", "cmd", "launch_google_meet_chrome", "config.json"), // ../cmd/launch_google_meet_chrome directory

	}

	config, err := utils.LoadConfig(configPaths)
	if err != nil {
		panic("ERROR!!! Cannot load config: " + err.Error())
	}

	d, err := utils.GetFileContents(credentialsPaths)
	if err != nil {
		panic("ERROR!! Cannot load credentials!! " + err.Error())
	}

	config.Credentials = d
	serverReady := make(chan struct{})

	// start the server
	go meettask.GRPCServer(ctx, config, serverReady)

	// Block until the server is ready - this fixes a case when getTasks sees it needs to launch a server yet the GRPC server is not up
	<-serverReady

	// get tasks that implement the interface
	t, err := meettask.GetTasks(config)
	if err != nil {
		panic(err)
	}
	// Convert the tasks into a cron task
	cron := tasks.NewCron(ctx, t, config)

	// refresh the task list
	go meettask.UpdateCronMeetings(ctx, cron)

	// do while
	go cron.Run()

	<-ctx.Done()
	logrus.Println("SHUTDOWN")
	//trace.Start(os.Stderr)
	os.Exit(0)
}
