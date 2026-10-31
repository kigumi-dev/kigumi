package vm

// blockJob: every field but fin belongs to the scheduler lock.
type blockJob struct {
	done    bool
	result  *obj
	failure any
	waiter  *task
	// waitEx keeps the awaiting task's executor from being declared stuck while the job runs.
	waitEx *executor
	fin    chan struct{}
}

func (m *Machine) offload(poolObj, work *obj) *obj {
	p := poolObj.opaqueData().(*blockingPool)
	p.inFlight.Add(1)
	job := &blockJob{fin: make(chan struct{})}
	lane := m.lane()
	go lane.runBlocking(job, work)
	return mkOpaque("Future", &future{job: job, pool: retain(poolObj)})
}

func (m *Machine) runBlocking(job *blockJob, work *obj) {
	var result *obj
	var failure any
	func() {
		defer func() { failure = recover() }()
		result = m.callValue(work, nil)
	}()
	m.release(work)
	m.sh.sched.Lock()
	job.done, job.result, job.failure = true, result, failure
	if ex := job.waitEx; ex != nil {
		ex.external--
		ex.cv.Broadcast()
		job.waitEx = nil
	}
	if w := job.waiter; w != nil {
		job.waiter = nil
		m.wake(w)
	}
	m.sh.sched.Unlock()
	close(job.fin)
}

func (m *Machine) pollJob(fu *future) (*obj, polled) {
	job := fu.job
	m.sh.sched.Lock()
	if !job.done {
		if job.waitEx == nil && m.cur != nil {
			job.waitEx = m.cur.ex
			job.waitEx.external++
		}
		job.waiter = m.cur
		m.sh.sched.Unlock()
		return nil, pending
	}
	m.sh.sched.Unlock()
	fu.done = true
	m.dropAdmission(fu)
	r := fu.job.result
	fu.job.result = nil
	if f := fu.job.failure; f != nil {
		m.rethrow(f)
	}
	return r, ready
}

// Dropping a pending result waits: blocking work cannot be interrupted and its captures must outlive it.
func (m *Machine) dropJob(fu *future) {
	job := fu.job
	fu.job = nil
	<-job.fin
	m.release(job.result)
	job.result = nil
}

func (m *Machine) dropAdmission(fu *future) {
	if fu.pool != nil {
		fu.pool.opaqueData().(*blockingPool).inFlight.Add(-1)
		m.release(fu.pool)
		fu.pool = nil
	}
}
