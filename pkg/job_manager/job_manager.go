package job_manager

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// ----- JobStatus -----

type JobStatus string

const (
	StatusRunning    JobStatus = "running"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusTerminated JobStatus = "terminated"
)

// ----- Log and its related methods/functions -----

type Log struct {
	Mutex sync.RWMutex
	Lines []string
}

func NewLog() *Log {
	l := &Log{}
	return l
}

func (log *Log) AppendLine(line string) {
	log.Mutex.Lock()
	defer log.Mutex.Unlock()
	log.Lines = append(log.Lines, line)
}

// ----- LogReader and its related methods/functions -----

type LogReader struct {
	CurrentPosition int
	log             *Log
}

func (r *LogReader) ReadNextLine(blocking bool) (string, bool) {
	log := r.log

	// We use read-locks because appending to a slice not thread safe. Since we have an append-only
	// log, there should be a way to refactor the code though to reduce the need for locking.
	log.Mutex.RLock()
	defer log.Mutex.RUnlock()

	for {
		if !blocking || r.CurrentPosition < len(log.Lines) {
			break
		}
		// TODO: Use syncronization primitives to avoid busy waiting
		log.Mutex.RUnlock()
		time.Sleep(10 * time.Millisecond)
		log.Mutex.RLock()
	}

	if r.CurrentPosition >= len(log.Lines) {
		return "", false
	}

	line := log.Lines[r.CurrentPosition]
	r.CurrentPosition++
	return line, true
}

// ----- Primary Interface: Job and its related methods/functions. -----

type Job struct {
	Cmd         *exec.Cmd
	Status      JobStatus
	Log         *Log
	Mutex       sync.RWMutex
}

func StartJob(command string, args ...string) (*Job, error) {
	cmd := exec.Command(command, args...)
	cmdOut, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmdErr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	job := &Job{
		Cmd:         cmd,
		Status:      StatusRunning,
		Log:         NewLog(),
	}

	go job._captureOutput(cmdOut, cmdErr)

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go job._monitorCompletion()

	return job, nil
}

func (job *Job) _captureOutput(stdout, stderr io.Reader) {
	stdoutBuf := bufio.NewReader(stdout)
	stderrBuf := bufio.NewReader(stderr)

	readNextLine := func(buf *bufio.Reader) {
		bytes, err := buf.ReadBytes('\n')
		if (err == nil || err == io.EOF) && len(bytes) > 0 {
			job.Log.AppendLine(string(bytes))
		}
	}

	for {
		job.Mutex.RLock()
		if job.Status != StatusRunning {
			job.Mutex.RUnlock()
			break
		}
		job.Mutex.RUnlock()

		readNextLine(stdoutBuf)
		readNextLine(stderrBuf)
	}
}

func (job *Job) _monitorCompletion() {
	err := job.Cmd.Wait()

	job.Mutex.Lock()
	defer job.Mutex.Unlock()

	if err != nil {
		job.Status = StatusFailed
	} else {
		job.Status = StatusCompleted
	}
}

func (job *Job) Stop() error {
	job.Mutex.Lock()
	defer job.Mutex.Unlock()

	if job.Status != StatusRunning {
		return errors.New("job is not running or has already completed")
	}

	if err := job.Cmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}

	job.Status = StatusTerminated
	return nil
}

func (job *Job) GetStatus() JobStatus {
	job.Mutex.Lock()
	defer job.Mutex.Unlock()
	return job.Status
}

func (job *Job) NewLogReader() LogReader {
	return LogReader{
		CurrentPosition: 0,
		log:             job.Log,
	}
}

func (job *Job) ReadAllLines() []string {
	logReader := job.NewLogReader()
	var lines []string
	for {
		line, ok := logReader.ReadNextLine(false)
		if !ok {
			break
		}
		lines = append(lines, string(line))
	}
	return lines
}
