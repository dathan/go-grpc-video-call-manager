package session

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/sirupsen/logrus"
)

/**
 * always give credit: https://github.com/perkeep/gphotos-cdp adaption
 */
type Session struct {
	parentContext context.Context
	parentCancel  context.CancelFunc
	profileDir    string // user data session dir. automatically created on chrome startup.
}

//NewSession a session to control the browser creation, creates a new browser if one is not running
func NewSession() (*Session, error) {

	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	dir += "/Session"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// path/to/whatever does not exist
		err = os.Mkdir(dir, 0755)
		if err != nil {
			return nil, err
		}

	}

	s := &Session{
		profileDir: dir,
	}
	return s, nil

}

// Build a NewSession to open a meet uri
func (s *Session) NewContext() (context.Context, context.CancelFunc) {
	// Let's use as a base for allocator options (It implies Headless)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.UserDataDir(s.profileDir),
	)

	// undo the three opts in chromedp.Headless() which is included in DefaultExecAllocatorOptions
	opts = append(opts, chromedp.Flag("headless", false))
	opts = append(opts, chromedp.Flag("hide-scrollbars", false))
	opts = append(opts, chromedp.Flag("mute-audio", false))
	opts = append(opts, chromedp.Flag("disable-gpu", false))
	opts = append(opts, chromedp.Flag("restore-on-startup", false))
	opts = append(opts, chromedp.Flag("start-fullscreen", true))
	opts = append(opts, chromedp.Flag("enable-automation", false))

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	s.parentContext = ctx
	s.parentCancel = cancel
	ctx, cancel = chromedp.NewContext(s.parentContext)
	return ctx, cancel
}

//AddTab return another tab to navigate to
func (s *Session) AddTab() (context.Context, context.CancelFunc) {
	return chromedp.NewContext(s.parentContext)
}

//Shutdown calls the parent context to cancel
func (s *Session) Shutdown() {
	logrus.Info("Session is shutting down")
	s.parentCancel()
}

// login navigates to https://photos.google.com/ and waits for the user to have
// authenticated (or for 2 minutes to have elapsed).
func (s *Session) Login(ctx context.Context) error {

	waitForLogin := time.Duration(10) * time.Minute
	//loggedin = $x("//*/a[@role=\"button\"]/img/..")
	loggedInCheck := "//*/a[@role=\"button\"]/img/.."

	var nodes []*cdp.Node
	return chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			logrus.Debugf("pre-navigate")
			return nil
		}),
		chromedp.Navigate("https://meet.google.com/"),
		// when we're not authenticated, the URL is actually
		// https://meet.google.com , so we rely on that to detect when we have
		// authenticated.
		chromedp.ActionFunc(func(ctx context.Context) error {
			tick := time.Second
			timeout := time.Now().Add(waitForLogin)
			var location string
			for {
				if time.Now().After(timeout) {
					return errors.New("timeout waiting for authentication")
				}

				if err := chromedp.Nodes(loggedInCheck, &nodes, chromedp.AtLeast(0)).Do(ctx); err != nil {
					return err
				}

				if len(nodes) >= 1 {
					return nil
				}

				if strings.Contains(location, "hs=") {
					return nil
				}

				logrus.Infof("Not yet authenticated, at: %v", location)
				time.Sleep(tick)
			}
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			logrus.Debugf("post-navigate")
			return nil
		}),
	)
}

// Open navigates to the meeturn uri and waits for the body to load.
// Then turns off the mic, turns off the camera and joins the meeting
func (s *Session) Open(ctx context.Context, meetURI string) error {
	return s.execute(ctx, "OPEN", s.navigateUrl(meetURI))
}

// navigateUrl opens the actual url
func (s *Session) navigateUrl(meetURI string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(meetURI),
		chromedp.WaitVisible("body", chromedp.ByQuery),
	}
}

// ApplySettings applies the preferences saved by the user such as shutting off mute and
func (s *Session) ApplySettings(ctx context.Context) error {

	// find the button that contains a body of text by pulling the document into a format that can search the body of the message
	// there are two ways that I'm thinking this can be done. goquery the node for the matching text, loop through each node and traverse the graph for the button that has the value
	// - going to use goquery
	//$x("/html/body//span/text()[contains(translate(., 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'),'join')]")
	selector := "//span/text()[contains(translate(., 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'),'join now')]/.."

	tasks := chromedp.Tasks{
		chromedp.Sleep(1 * time.Second),
		input.DispatchKeyEvent(input.KeyDown).WithModifiers(input.ModifierMeta).WithKey(`d`),
		input.DispatchKeyEvent(input.KeyDown).WithModifiers(input.ModifierMeta).WithKey(`e`),
		chromedp.Sleep(1 * time.Second),
	}

	if err := s.execute(ctx, "SETTINGS", tasks); err != nil {
		logrus.Warnf("SETTINGS ERROR: %s\n", err)
		return err
	}

	if err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			logrus.Debugf("pre-click")
			err := chromedp.Click(selector, chromedp.BySearch).Do(ctx)
			if err != nil {
				return err
			}
			logrus.Debugf("post-click")
			return nil
		})); err != nil {
		logrus.Debugf("CLICK ERROR: %s\n", err)
		return err

	}

	return nil

}

// execute runs tasks
func (s *Session) execute(ctx context.Context, actionType string, actions chromedp.Tasks) error {

	actions = append(
		//todo routine to return pre/post tasks for every execute
		chromedp.Tasks{

			chromedp.ActionFunc(
				func(ctx context.Context) error {
					logrus.Debugf("PRE-%s\n", actionType)
					return nil
				}),
		},
		actions...)

	actions = append(actions, chromedp.Tasks{

		chromedp.ActionFunc(
			func(ctx context.Context) error {
				logrus.Debugf("POST-%s\n", actionType)
				return nil
			}),
	})

	err := chromedp.Run(ctx, actions)
	return err

}

// wait for the browser to exit
func (s *Session) Wait(ctx context.Context) {
	logrus.Infof("Waiting for the browser to exit")
	defer s.Shutdown()
	for {
		select {
		case <-ctx.Done():
			logrus.Infof("Browser Context is done exiting")
			return
		case <-s.parentContext.Done():
			logrus.Infof("Browser Parent context is done existing")
			return
		}
	}
}
