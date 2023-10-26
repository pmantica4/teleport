package job_manager

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartJob(t *testing.T) {
	job, err := StartJob("echo", "Hello World!")
	if err != nil {
		t.Fatalf("Failed to start job: %v", err)
	}

	if job.GetStatus() != StatusRunning {
		t.Fatalf("Expected job status to be 'running', but got: %v", job.GetStatus())
	}
}

func TestCaptureStdout(t *testing.T) {
	message := "Hello from stdout!"
	job, _ := StartJob("echo", message)

	logReader := job.NewLogReader()
	line, _ := logReader.ReadNextLine(true)
	if !strings.Contains(line, message) {
		t.Fatalf("Expected to capture message: %v, got: %v", message, line)
	}
}

func TestCaptureStderr(t *testing.T) {
	message := "Hello from stderr!"
	job, _ := StartJob("bash", "-c", ">&2 echo "+message)
	logReader := job.NewLogReader()
	line, _ := logReader.ReadNextLine(true)

	if !strings.Contains(line, message) {
		t.Fatalf("Expected to capture message: %v, got: %v", message, line)
	}
}

func TestStopRunningJob(t *testing.T) {
	job, _ := StartJob("sleep", "10")

	err := job.Stop()
	if err != nil {
		t.Fatalf("Failed to stop job: %v", err)
	}

	if job.GetStatus() != StatusTerminated {
		t.Fatalf("Expected job status to be 'terminated', but got: %v", job.GetStatus())
	}
}

func TestStopCompletedJob(t *testing.T) {
	job, _ := StartJob("echo", "Hello World!")
	time.Sleep(1 * time.Second)

	err := job.Stop()
	if err == nil {
		t.Fatal("Expected error when trying to stop a completed job")
	}
}

func TestGetJobStatus(t *testing.T) {
	job, _ := StartJob("sleep", "2")
	if job.GetStatus() != StatusRunning {
		t.Fatalf("Expected job status to be 'running', but got: %v", job.GetStatus())
	}

	time.Sleep(3 * time.Second)
	if job.GetStatus() != StatusCompleted {
		t.Fatalf("Expected job status to be 'completed', but got: %v", job.GetStatus())
	}
}

func TestReadAllLines(t *testing.T) {
	job, _ := StartJob("bash", "-c", "echo line1 && echo line2 && echo line3")
	time.Sleep(1 * time.Second)

	lines := job.ReadAllLines()
	expectedLines := []string{"line1\n", "line2\n", "line3\n"}
	for i, line := range lines {
		if line != expectedLines[i] {
			t.Fatalf("Expected line: %v, got: %v", expectedLines[i], line)
		}
	}
}

// TestJobCreationAndStatus verifies that a job can be created and has the correct status after completion.
func TestJobCreationAndStatus(t *testing.T) {
	job, err := StartJob("echo", "Hello")
	if err != nil {
		t.Fatalf("Failed to start the job: %v", err)
	}
	time.Sleep(2 * time.Second) // Giving some time for the job to run
	status := job.GetStatus()
	if status != StatusCompleted {
		t.Fatalf("Expected job status to be %v, got %v", StatusCompleted, status)
	}
}

// TestJobStopping verifies that a long-running job can be manually stopped.
func TestJobStopping(t *testing.T) {
	job, err := StartJob("sleep", "10") // A job that sleeps for 10 seconds
	if err != nil {
		t.Fatalf("Failed to start the job: %v", err)
	}
	time.Sleep(1 * time.Second) // Wait for 1 second
	err = job.Stop()
	if err != nil {
		t.Fatalf("Failed to stop the job: %v", err)
	}
	status := job.GetStatus()
	if status != StatusTerminated {
		t.Fatalf("Expected job status to be %v, got %v", StatusTerminated, status)
	}
}

// TestNonBlockingLogReading tests reading logs in non-blocking mode.
func TestNonBlockingLogReading(t *testing.T) {
	job, err := StartJob("echo", "Hello")
	if err != nil {
		t.Fatalf("Failed to start the job: %v", err)
	}
	time.Sleep(1 * time.Second)
	logReader := job.NewLogReader()
	line, ok := logReader.ReadNextLine(false)
	if !ok || !strings.Contains(line, "Hello") {
		t.Fatalf("Expected to read 'Hello', got: %v", line)
	}
	line, ok = logReader.ReadNextLine(false)
	if ok {
		t.Fatalf("Expected no more lines, but got: %v", line)
	}
}

// TestMultipleConcurrentReaders verifies that multiple readers can read from the job logs simultaneously.
func TestMultipleConcurrentReaders(t *testing.T) {
	message := "Concurrent read test"
	job, err := StartJob("echo", message)
	if err != nil {
		t.Fatalf("Failed to start the job: %v", err)
	}

	var wg sync.WaitGroup
	readerCount := 5

	for i := 0; i < readerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reader := job.NewLogReader()
			next_line, _ := reader.ReadNextLine(true)
			if !strings.Contains(next_line, message) {
				t.Errorf("Expected to capture message: %v, but got: %v", message, next_line)
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentWriteAndRead verifies simultaneous writing (by the job) and reading logs.
func TestConcurrentWriteAndRead(t *testing.T) {
	job, err := StartJob("bash", "-c", "for i in {1..5}; do echo $i; sleep 1; done")
	if err != nil {
		t.Fatalf("Failed to start the job: %v", err)
	}

	var wg sync.WaitGroup
	readerCount := 5

	for i := 0; i < readerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logReader := job.NewLogReader()
			for j := 1; j <= 5; j++ {
				line, ok := logReader.ReadNextLine(true) // true indicates it's blocking
				if !ok || !strings.Contains(line, fmt.Sprint(j)) {
					t.Errorf("Expected to read number %d, but got: %v", j, line)
				}
			}
		}()
	}

	wg.Wait()
}

// TestLongRunningJobWithMultipleReaders verifies that multiple readers can continue to get updated logs as the job runs.
func TestLongRunningJobWithMultipleReaders(t *testing.T) {
	job, err := StartJob("bash", "-c", "for i in {1..10}; do echo $i; sleep 2; done")
	if err != nil {
		t.Fatalf("Failed to start the job: %v", err)
	}

	var wg sync.WaitGroup
	readerCount := 3

	for i := 0; i < readerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logReader := job.NewLogReader()
			for j := 1; j <= 10; j++ {
				line, ok := logReader.ReadNextLine(true)
				if !ok || !strings.Contains(line, fmt.Sprint(j)) {
					t.Errorf("Expected to read number %d, but got: %v", j, line)
				}
				time.Sleep(1 * time.Second) // Sleeping to simulate staggered reading
			}
		}()
	}

	wg.Wait()
}
