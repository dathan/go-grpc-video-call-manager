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
}

// Create a new timed task
func NewCron(ctx context.Context, mis SequentialTasks) *Cron {
	return &Cron{
		ordered:       mis,
		parentContext: ctx,
		taskChan:      make(chan SequentialTasks),
		tLock:         &sync.Mutex{},
		timer:         time.NewTimer(time.Hour * 86000), // set a default timer to fix a race condition
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

		c.timer = time.NewTimer(d)
		break
	}

	if c.timer == nil {
		logrus.Warnf("Timer is nil\n")
		return
	}
}

func (c *Cron) Loop() {

	//TODO: revidew this, feels that run is doing to much and this timer is adding an AND to the method yet it is cronlist so I'm conflicted
	// block for the timer
	for {
		select { // listen for an update to the calendar
		case tsk := <-c.taskChan:
			logrus.Info("Recieved Update to TaskChan - stoping timers, updating list, rerunning.")
			c.timer.Stop()
			c.internalUpdate(tsk)
			c.Run()
		case <-c.parentContext.Done():
			c.timer.Stop()
			return
		case <-c.timer.C:
			c.timer.Stop()
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
