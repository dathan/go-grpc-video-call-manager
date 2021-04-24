package tasks

import (
	"context"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
)

// MAGIC_DELTA
const MAGIC_DELTA = float64(600) // seconds

//Things that satisfy this interface can be executed as a Task
type Task interface {
	Start() time.Time
	End() time.Time
	Execute() error //A Task has to be able to be run
}

// Tasks that are sequentially executed
type SequentialTasks []Task

// Launch a bunch of sequentail tasks when their time is due
type Cron struct {
	ordered       SequentialTasks
	parentContext context.Context
	taskChan      chan SequentialTasks
	tLock         *sync.Mutex
}

// Create a new timed task
func NewCronTask(ctx context.Context, mis SequentialTasks) *Cron {
	return &Cron{
		ordered:       mis,
		parentContext: ctx,
		taskChan:      make(chan SequentialTasks),
		tLock:         &sync.Mutex{},
	}
}

// Run a Crontask
func (c *Cron) Run() {

	var timer *time.Timer
	if len(c.ordered) == 0 {
		logrus.Info("Tasks are finished")
		return
	}

	// c.ordered is order in time.
	for _, task := range c.ordered {

		s := time.Now().Add(time.Second * time.Duration(MAGIC_DELTA)) // if now+10min is after task start
		if s.After(task.Start()) {

			if err := task.Execute(); err != nil {
				logrus.Warnf("Task: %v - ERROR - %s\n", task, err)
			}

			c.internalUpdate(c.ordered[1:])
			continue
		}

		//start - now() == delta
		d := task.Start().Sub(time.Now().Add(time.Second * time.Duration(MAGIC_DELTA))) // start 10 mins early

		logrus.Infof("TASK[ %+v ] - Timer Execution: %f seconds", task, d.Seconds())

		timer = time.NewTimer(d)
		break
	}

	if timer == nil {
		logrus.Warnf("Timer is nil\n")
		return
	}

	// block for the timer
	for {
		select { // listen for an update to the calendar
		case tsk := <-c.taskChan:
			logrus.Info("Recieved Update to TaskChan - stoping timers, updating list, rerunning.")
			timer.Stop()
			c.internalUpdate(tsk)
			c.Run()
		case <-c.parentContext.Done():
			timer.Stop()
			return
		case <-timer.C:
			timer.Stop()
			c.Run()
		}
	}
}

// Update the tasks channel so listeners can execute the new job order
func (c *Cron) Update(st SequentialTasks) {

	if cmp.Equal(c.ordered, st) == false {
		logrus.Info("Updating the channel to replace the task list")
		c.taskChan <- st
		logrus.Info("Update Sent")
	}
}

func (c *Cron) internalUpdate(st SequentialTasks) {
	c.tLock.Lock()
	c.ordered = st
	c.tLock.Unlock()
}
