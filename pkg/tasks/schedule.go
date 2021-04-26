package tasks

import (
	"context"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
)

// TODO: turn into a pref
const MAGIC_DELTA = float64(600) // seconds

//Things that satisfy this interface can be executed as a Task
type Task interface {
	Start() time.Time
	Name() string
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
	timer         *time.Timer
	timerOn       bool
}

// Create a new timed task
func NewCron(ctx context.Context, mis SequentialTasks) *Cron {
	return &Cron{
		ordered:       mis,
		parentContext: ctx,
		taskChan:      make(chan SequentialTasks),
		tLock:         &sync.Mutex{},
		timer:         time.NewTimer(time.Hour * 86000), // set a default timer to fix a race condition
		timerOn:       false,
	}
}

// Run the Crontask list
func (c *Cron) Run() {

	if len(c.ordered) == 0 {
		logrus.Info("Tasks are finished")
		return
	}

	// c.ordered is order in time.
	for _, task := range c.ordered {

		s := time.Now().Add(time.Second * time.Duration(MAGIC_DELTA)) // if now+10min is after task start
		if s.After(task.Start()) {                                    // handle if the task already started

			if err := task.Execute(); err != nil {
				logrus.Warnf("Task: %v - ERROR - %s\n", task, err)
			}

			c.internalUpdate(c.ordered[1:])
			continue
		}

		//TODO: make preference of scheduling the task buffer
		k := task.Start().Add(-time.Second * time.Duration(MAGIC_DELTA)) // start 10 mins early
		d := k.Sub(time.Now())
		logrus.Infof("TASK START[ %s => %+v ] - Timer Execution: %f seconds: %s", task.Name(), task.Start(), d.Seconds(), d.String())

		c.setTimer(time.NewTimer(d))

		break
	}

	if c.timer == nil {
		logrus.Warnf("Timer is nil\n")
		return
	}
}

// Wait for the timer to finish to launch the next item in the queue
func (c *Cron) Loop() {

	// block for the timer
	for {
		select { // listen for an update to the calendar
		case tsk := <-c.taskChan:
			logrus.Info("Recieved Update to TaskChan - stoping timers, updating list, rerunning.")
			c.stopTimer()
			c.internalUpdate(tsk)
			c.Run()
		case <-c.parentContext.Done():
			c.stopTimer()
			return
		case <-c.timer.C:
			c.stopTimer()
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

// abstract the lock when changing the SequentialTask
func (c *Cron) internalUpdate(st SequentialTasks) {
	c.tLock.Lock()
	c.ordered = st
	c.tLock.Unlock()
}

// helper method to set a timer
func (c *Cron) setTimer(t *time.Timer) {
	c.stopTimer()
	c.timer = t
	c.timerOn = true
}

// helper method to stop a stimer
func (c *Cron) stopTimer() {
	if c.timerOn {
		c.timer.Stop()
	}
	c.timerOn = false
}
