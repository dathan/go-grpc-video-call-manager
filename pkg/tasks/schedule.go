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
	currentTask   *Task
}

// Create a new timed task
func NewCron(ctx context.Context, mis SequentialTasks) *Cron {
	return &Cron{
		ordered:       mis,
		parentContext: ctx,
		taskChan:      make(chan SequentialTasks),
		tLock:         &sync.Mutex{},
	}
}

// Run the Crontask or schedule to run it later.
func (c *Cron) Run() {

	var timer *time.Timer // each run will have its own timer to loop

	if len(c.ordered) == 0 {
		logrus.Info("Tasks are finished")
		return
	}

	// c.ordered is order in time and we will either execute the task or schedule the task to run
	for _, task := range c.ordered {

		if c.currentTask != nil && cmp.Equal(task, c.currentTask) {
			logrus.Warnf("Race condition task: %v", c.currentTask)
			c.currentTask = nil
			continue
		}

		s := time.Now().Add(time.Second * time.Duration(MAGIC_DELTA)) // if now+10min is after task start

		if s.After(task.Start()) { // handle if the task already started
			c.currentTask = &task // todo: lock
			if err := task.Execute(); err != nil {
				logrus.Warnf("Task: %v - ERROR - %s\n", task, err)
			}
			continue // notice we will continue iterating the next task lisk
		}

		c.currentTask = nil

		//TODO: make preference of scheduling the task buffer
		d := task.Start().Sub(s)

		logrus.Infof("NEW SCHEDULED START[ %s => %+v ] - Timer Execution: %f seconds", task.Name(), task.Start(), d.Seconds())
		timer = time.NewTimer(d)
		break
	}

	if timer == nil {
		logrus.Warnf("Timer is nil\n")
		return
	}

	// each run will have its own schedule to run
	c.wait(timer)
}

// Update the tasks channel so listeners can execute the new job order
func (c *Cron) Update(st SequentialTasks) {

	logrus.Infof("Next Task: %s => Starts: %s", c.ordered[0].Name(), c.ordered[0].Start().Sub(time.Now()))

	if c.currentTask != nil {
		logrus.Infof("Current Task: %s => Started: %s", (*c.currentTask).Name(), (*c.currentTask).Start())
	} else {
		logrus.Infof("Current Task is not set")
	}

	if !cmp.Equal(c.ordered, st) {
		logrus.Info("Updating the channel to replace the task list")
		c.taskChan <- st
		logrus.Info("Update Sent")
	}
}

// Wait for the timer to finish to launch the next item in the queue
func (c *Cron) wait(t *time.Timer) {

	// notes channels are a queue of values, and selects are blocking by default unless a default case is used.
	select {
	case tsk := <-c.taskChan: // listen for an update to the calendar
		logrus.Infof("Recieved Update to TaskChan - stoping timers, updating list, rerunning.")
		t.Stop()
		c.internalUpdate(tsk)
		go c.Run()
	case <-c.parentContext.Done(): // listen for an exit
		logrus.Info("Parent is done")
		t.Stop()
	case <-t.C:
		go c.Run()
	}
}

// abstract the lock when changing the SequentialTask
func (c *Cron) internalUpdate(st SequentialTasks) {
	c.tLock.Lock()
	c.ordered = st
	c.tLock.Unlock()
}
