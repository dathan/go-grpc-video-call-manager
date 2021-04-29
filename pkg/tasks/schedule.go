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
	inProgress    bool
	currentTask   *Task
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

// Run the Crontask or schedule to run it later.
func (c *Cron) Run() {

	for c.inProgress {
		logrus.Infof("Waiting on the existing task to finish : %+v", c.currentTask)
		time.Sleep(30 * time.Second)
		continue
	}

	if len(c.ordered) == 0 {
		logrus.Info("Tasks are finished")
		return
	}

	// c.ordered is order in time.
	for _, task := range c.ordered {

		if c.currentTask != nil && cmp.Equal(task, c.currentTask) {
			logrus.Warnf("Race condition task: %s", c.currentTask)
			c.currentTask = nil
			continue
		}

		c.currentTask = &task                                         // todo: lock
		s := time.Now().Add(time.Second * time.Duration(MAGIC_DELTA)) // if now+10min is after task start

		if s.After(task.Start()) { // handle if the task already started
			c.inProgress = true
			if err := task.Execute(); err != nil {
				logrus.Warnf("Task: %v - ERROR - %s\n", task, err)
			}
			continue // notice we will continue iterating the next task lisk
		}

		c.inProgress = false

		//TODO: make preference of scheduling the task buffer
		d := task.Start().Sub(s)
		logrus.Infof("NEW SCHEDULED START[ %s => %+v ] - Timer Execution: %f seconds: %s", task.Name(), task.Start(), d.Seconds(), d.String())
		c.startTimer(d)
		return
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
		// notes channels are a queue of values, and selects are blocking by default unless a default case is used.
		select {
		case tsk := <-c.taskChan: // listen for an update to the calendar
			logrus.Infof("Recieved Update to TaskChan - stoping timers, updating list, rerunning.")
			c.internalUpdate(tsk)
			go c.Run() // just run it and do not block the other channels
		case <-c.parentContext.Done(): // listen for an exit
			logrus.Info("Parent is done")
			return
		case <-c.timer.C:
			go c.Run()
		}
	}

}

// Update the tasks channel so listeners can execute the new job order
func (c *Cron) Update(st SequentialTasks) {

	logrus.Infof("Next Task: %s => Starts: %s", c.ordered[0].Name(), c.ordered[0].Start().Sub(time.Now()))

	if c.currentTask != nil {
		logrus.Infof("Current Task: %s => Starts: %s [timer? %s]", (*c.currentTask).Name(), (*c.currentTask).Start(), c.timerOn)
	} else {
		logrus.Infof("Current Task is not set")
	}

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
func (c *Cron) startTimer(d time.Duration) {
	logrus.Info("attempting to start the timer")
	c.stopTimer()
	c.tLock.Lock()
	defer c.tLock.Unlock()

	if c.timer.Reset(d) { // timer is already set and need to stop then reset a timer to make it work
		c.timerOn = true
		logrus.Info("timer reset")
		return
	}

	logrus.Warn("Failed to reset the timer")
}

// helper method to stop a stimer
func (c *Cron) stopTimer() {
	c.tLock.Lock()
	defer c.tLock.Unlock()
	if !c.timer.Stop() {
		logrus.Warn("TIMER has failed to stop")
		//<-c.timer.C - the spec says drain the timer but typically the timer is not set and there is no way to test if the channel is open
		c.timer.Stop()
		logrus.Warn("Trying to drain")
		return
	}
	logrus.Info("TIMER HAS STOPPED")
	c.timerOn = false
}
