package main

import (
	"fmt"

	"github.com/dathan/go-grpc-video-call-manager/pkg/calendar"

	"github.com/dathan/go-grpc-video-call-manager/pkg/session"
)

func main() {
	cal := &calendar.CalService{}
	meetings, err := cal.GetUpcomingMeetings()

	if err != nil {
		panic(err)
	}

	urlstr := meetings[0].Uri
	fmt.Printf("About to navigate to URI: %s\n", urlstr)

	meet, err := session.NewSession()
	if err != nil {
		panic(err)
	}

	ctx, cancel := meet.NewContext()
	defer cancel()

	err = meet.Login(ctx)

	if err != nil {
		panic(err)
	}

	err = meet.Open(ctx, urlstr)

	if err != nil {
		panic(err)
	}

	err = meet.ApplySettings(ctx)
	if err != nil {
		panic(err)
	}

	meet.Wait(ctx)

}
