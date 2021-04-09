package tasks

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// MAGIC_DELTA
const MAGIC_DELTA = float64(1600)

//Things that satisfy this interface can be executed as a Task
type Task interface {
	When() time.Time
	End() time.Time
	Execute() error //A Task has to be able to be run
}

// Tasks that are sequentially executed
type SequentialTasks []Task

// Launch a bunch of sequentail tasks when their time is due
type Cron struct {
	ordered       SequentialTasks
	parentContext context.Context
}

// Create a new timed task
func NewCronTask(ctx context.Context, mis SequentialTasks) *Cron {
	return &Cron{
		ordered:       mis,
		parentContext: ctx,
	}
}

// Run a Crontask
func (c *Cron) Run() {

	var timer *time.Timer
	if len(c.ordered) == 0 {
		return
	}

	// c.ordered is order in time.
	for _, task := range c.ordered {
		s := time.Now().Add(time.Second * time.Duration(MAGIC_DELTA))

		if s.After(task.When()) {
			if err := task.Execute(); err != nil {
				logrus.Warnf("Task: %v - ERROR - %s\n", task, err)
				return
			}
			c.ordered = c.ordered[1:]
			continue
		}

		d := task.When().Sub(time.Now())

		logrus.Infof("Timer Execution: %d seconds Task: %v", d.Seconds(), task)

		timer = time.NewTimer(d)
		break
	}

	if timer == nil {
		return
	}

	// block for the timer
	for {
		select {
		case <-c.parentContext.Done():
			timer.Stop()
			return
		case <-timer.C:
			timer.Stop()
			c.Run()
			return
		}
	}
}
