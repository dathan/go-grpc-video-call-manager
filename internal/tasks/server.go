package tasks

import (
	"context"
	"fmt"
	"net"

	"github.com/chromedp/chromedp"
	"github.com/dathan/go-grpc-video-call-manager/internal/utils"
	"github.com/dathan/go-grpc-video-call-manager/pkg/manager"
	"github.com/dathan/go-grpc-video-call-manager/pkg/session"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type server struct {
	manager.UnimplementedOpenMeetUrlServer
}

// OpenMeetUrl for the local server open the meet url
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

	ctx1, cancel1 := chromedp.NewContext(ctx)
	defer cancel1()

	err = chromedp.Run(ctx1, chromedp.Navigate("https://calendar.google.com/calendar/u/0/r?pli=1"))
	//err = meet.Open(ctx1, "https://calendar.google.com/calendar/u/0/r?pli=1")
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

	//Waiting means you need to wait for the browser process to exit
	//TODO - wait for the tab to exit so you can avoid the browser context wait lock
	meet.Wait(ctx)

	ret := &manager.Status{
		Ok: true,
	}

	return ret, nil
}

// GRPCServer is launched via a go routine
func GRPCServer(ctx context.Context, config *utils.Config, serverReady chan<- struct{}) {
	port := config.Port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logrus.Errorf("could not listen to port %d: %v", port, err)
		panic(err)
	}

	logrus.Infof("GRPCServer starting localhost:%d\n", port)

	s := grpc.NewServer()
	manager.RegisterOpenMeetUrlServer(s, server{})

	go func(s *grpc.Server, lis net.Listener) {
		err := s.Serve(lis)
		if err != nil {
			logrus.Errorf("could not serve: %v", err)
		}
	}(s, lis)

	// Signal that the server is ready
	serverReady <- struct{}{}

	<-ctx.Done()
	logrus.Infoln("GRPC-Server is shutting down")
	s.GracefulStop()
}
