package job

import (
    "os/exec"
    "sync"
    "time"
)

type Job struct {
    ID            string
    Cmd           *exec.Cmd
    CreationTime  time.Time
    LastUpdated   time.Time
    Owner         string
    OutputBuffer  []byte
    Mutex         sync.Mutex
}

func NewJob(command string, args ...string) *Job {
    return &Job{
        ID:           generateUniqueID(),
        Cmd:          exec.Command(command, args...),
        CreationTime: time.Now(),
    }
}

// ... Other methods like Start, Stop, GetStatus, GetOutput
