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

	if len(c.ordered) == 0 {
		logrus.Info("Tasks are finished")
		return
	}

	// c.ordered is order in time and we will either execute the task or schedule the task to run
	for _, task := range c.ordered {

		select {
		case <-c.taskChan: // listen for an update to the calendar
			logrus.Infof("Task chan in loop returning")
			return
		default:
			logrus.Infof("Looking at ask: %+v", task)

		}

		s := time.Now().Add(time.Second * time.Duration(MAGIC_DELTA)) // if now+10min is after task start
		if s.After(task.Start()) {                                    // handle if the task already started

			c.currentTask = &task
			if err := task.Execute(); err != nil {
				logrus.Warnf("Task: %v - ERROR - %s\n", task, err)
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
		c.wait(timer)
		break
	}

	logrus.Info("Run finished")

}

// Update the tasks channel so listeners can execute the new job order
func (c *Cron) Update(st SequentialTasks) {

	logrus.Infof("Next Task: %s => Starts: %s", c.ordered[0].Name(), c.ordered[0].Start().Sub(time.Now()))

	st = c.pruneBeforeCurrentTask(st)

	if !cmp.Equal(c.ordered, st) {
		logrus.Info("Updating the channel to replace the task list")
		c.taskChan <- st // note this will block if a select has not been established to recieve the update
		logrus.Info("Update Sent")
	}
}

func (c *Cron) pruneBeforeCurrentTask(tsks SequentialTasks) SequentialTasks {

	if c.currentTask != nil {
		logrus.Infof("Current Task: %s => Started: %s", (*c.currentTask).Name(), (*c.currentTask).Start())
		for i := len(tsks) - 1; i >= 0; i-- {
			// note this should look at the status of a job running
			if tsks[i].Start().Before((*c.currentTask).End()) {
				tsks = append(tsks[:i], tsks[i+1:]...)
			}
		}
	} else {
		logrus.Infof("Current Task is not set")
	}

	return tsks

}

// Wait for the timer to finish to launch the next item in the queue
func (c *Cron) wait(t *time.Timer) {

	logrus.Info("Wating for a change")

	defer logrus.Info("Wait finished")
	// notes channels are a queue of values, and selects are blocking by default unless a default case is used.
	select {
	case tsk := <-c.taskChan: // listen for an update to the calendar
		logrus.Infof("Recieved Update to TaskChan - stoping timers, updating list, rerunning.")
		t.Stop()
		c.internalUpdate(tsk)
		go c.Run()
		return
	case <-c.parentContext.Done(): // listen for an exit
		logrus.Info("Parent is done")
		t.Stop()
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
