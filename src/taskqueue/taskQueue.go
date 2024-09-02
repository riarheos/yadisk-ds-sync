package taskqueue

import "sync"

type TaskQueue struct {
	maxWorkers   int
	curWorkers   int
	stopOnErrors bool
	queue        []Task
	lock         *sync.Mutex
	cond         *sync.Cond
}

type Task func() error

func NewTaskQueue(workers int, stopOnErrors bool) *TaskQueue {
	lock := &sync.Mutex{}
	return &TaskQueue{
		maxWorkers:   workers,
		stopOnErrors: stopOnErrors,
		queue:        make([]Task, 0),
		lock:         lock,
		cond:         sync.NewCond(lock),
	}
}

func (tq *TaskQueue) Run() error {
	var err error

	for {
		tq.lock.Lock()
		for (tq.curWorkers == tq.maxWorkers) ||
			(len(tq.queue) == 0 && tq.curWorkers > 0) ||
			(tq.stopOnErrors && err != nil && tq.curWorkers > 0) {
			tq.cond.Wait()
		}

		if (len(tq.queue) == 0) || (tq.stopOnErrors && err != nil) {
			tq.lock.Unlock()
			return err
		}

		tq.curWorkers++
		task := tq.queue[0]
		tq.queue = tq.queue[1:]

		go func(task Task) {
			te := task()

			tq.lock.Lock()
			tq.curWorkers--
			if te != nil {
				err = te
			}
			tq.cond.Signal()
			tq.lock.Unlock()
		}(task)

		tq.lock.Unlock()
	}
}

func (tq *TaskQueue) Push(task Task) {
	tq.lock.Lock()
	tq.queue = append(tq.queue, task)
	tq.lock.Unlock()
}
