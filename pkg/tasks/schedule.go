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
	lastTask      *Task
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

	go c.listenForUpdates()

	if len(c.ordered) == 0 {
		logrus.Warn("Tasks are finished")
		return
	}

	// from a golang concurrency perspective range produces a write to assign it to task.
	c.tLock.Lock()

	// c.ordered is order in time and we will either execute the task or schedule the task to run
	for _, task := range c.ordered {

		if c.lastTask != nil && (*c.lastTask).End().After(task.End()) {
			logrus.Warnf("Skipping Task: %v because lastTask: %v", task, (*c.lastTask))
			continue
		}

		logrus.Infof("Looking at task: %+v", task)

		s := time.Now().Add(time.Second * time.Duration(MAGIC_DELTA)) // if now+10min is after task start
		if s.After(task.Start()) {                                    // handle if the task already started

			c.currentTask = &task

			if err := task.Execute(); err != nil {
				logrus.Warnf("Task: %s - ERROR - %s\n", task, err)
			}

			c.currentTask = nil
			continue // notice we will continue iterating the next task lisk
		}

		c.currentTask = nil
		c.lastTask = &task

		//TODO: make preference of scheduling the task buffer
		d := task.Start().Sub(s)
		logrus.Infof("NEW SCHEDULED START[ %s => %+v ] - Timer Execution: %f seconds -> %s", task.Name(), task.Start(), d.Seconds(), time.Now().Add(d))
		timer = time.NewTimer(d)
		break
	}

	c.tLock.Unlock()

	if timer == nil {
		logrus.Warn("IMPOSSIBLE ERROR: Timer is nil sequential order is bogus. Run finished")
		return
	}

	c.wait(timer)
	logrus.Info("Run finished")

}

// Update the tasks channel so listeners can execute the new job order
func (c *Cron) Update(st SequentialTasks) {

	//logrus.Infof("Next Task: %s => Starts: %s", c.ordered[0].Name(), c.ordered[0].Start().Sub(time.Now()))

	if !cmp.Equal(c.ordered, st) {
		logrus.Info("Updating the channel to replace the task list")
		c.taskChan <- st // note this will block if a select has not been established to recieve the update
		logrus.Info("Update Sent")
	}
}

// if execute is not called we need to still listen for the taskChan updates to reset the array otherwise there is a deadlock
func (c *Cron) listenForUpdates() {

	logrus.Info("listening for update")
	defer func() {
		logrus.Info("listening for update FINISHED!")
	}()
	select {
	case tsk := <-c.taskChan: // listen for an update to the calendar
		logrus.Infof("Recieved Update to TaskChan - stoping timers, updating list, rerunning.")
		c.internalUpdate(tsk)
		go c.Run()
		return
	case <-c.parentContext.Done(): // listen for an exit
		logrus.Info("Parent is done")
		return
	}
}

// Wait for the timer to finish to launch the next item in the queue
func (c *Cron) wait(t *time.Timer) {

	logrus.Info("Waiting for a change")

	defer func() {
		logrus.Info("Wait finished, stoping the timer")
		if !t.Stop() {
			logrus.Warn("Unable to stop the timer trying to drain")
			<-t.C
			logrus.Warn("Drain Finished")
		}
		logrus.Info("timer stopped")
	}()

	// notes channels are a queue of values, and selects are blocking by default unless a default case is used.
	select {
	case <-c.taskChan: // listen for an update to the calendar
		logrus.Infof("Timer task chan detected update")
		return
	case <-c.parentContext.Done(): // listen for an exit
		logrus.Info("Parent is done")
		return
	case <-t.C:
		logrus.Infof("Timer executed for Task: %s", c.ordered[0])
		go c.Run()
		return
	}
}

// abstract the lock when changing the SequentialTask
func (c *Cron) internalUpdate(st SequentialTasks) {
	c.tLock.Lock()
	c.ordered = st
	c.tLock.Unlock()
}
