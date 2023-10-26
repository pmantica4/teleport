
# Remote Job Executor

## **Introduction**
This document outlines the design approach for developing a remote job executor that provides functionalities such as job creation, monitoring, control, and secure data transfer. Our primary objective is to design a system that is secure, efficient, and user-friendly.

## **Scope**

In this design, we aim to implement the level 5 requirements. This means that we need to implement:

1. A CLI for communicating with the API server
2. A library that allows clients to create jobs with resource control and isolation and allows multiple clients to stream a job's output.
3. A gRPC server with mTLS authentication and a simple authorization scheme.   


## **Design Approach**

### **1. Command-Line User Experience (CLI UX)**

The CLI will allow users to start and stop a job, query a job's state and stream their output. Below are some example commands. For job ids we are using the 12 character UUIDs. We are not using sequential numerical ids to prevent malicious users from guessing job ids.   

* **Starting a Job:**  
  ```
  $ cli start echo "Hello World" 
  Started job with ID: 4d5c9fa91db4
  ```

* **Starting a Job with flags/options:**
  ```
  # Per convention -- indicates the end of command options
  $ cli start -- bash -c "echo Hello World"
  Started job with ID: a2f3d7865b4e
  ```

* **Stopping a Job:**  
  ```
  $ cli stop 4d5c9fa91db4
  ```

* **Query Job Status:**
  ```
  $ cli status 4d5c9fa91db4
  running, completed, failed or terminated 
  ```

* **Viewing Job Output:**  
  ```
  $ cli log 4d5c9fa91db4
  ```

* **Streaming Job Ouput:**
  ```
  $ cli log -f 4d5c9fa91db4
  ```

#### **Future CLI Improvements**

It's worth mentioning that our CLI UX is basic and we would probably want to build something better in production. To improve our user CLI experience we could add:

* **Job Listing**
  ```
  $ cli ls
  ```

* **Job Naming**
  ```
  $ cli start -n hello_world -- echo Hello World 
  ```

For now, we'll leave these two improvements for the future to reduce our implementation scope. 

### **2. Library**

#### **Interface**

The library will expose the following functions as the primary interface:

```go
type struct Job {
  id
  <internal_fields>
}
type struct LogReader {
  <internal_fields>
}

// Managing jobs and queueing job status
StartJob(command string, args ...string) (*Job, error)
func (job *Job) Stop() error
func (job *Job) GetStatus() string

// Reading job output
func (job *Job) GetNewLogReader() (*LogReader, error)
func (reader *LogReader) GetNextLineBlocking()
func (reader *LogReader) GetNextLineNonBlocking()
```
#### **Job Life Cycle**

At a high level when creating a job we:
1. Configure cgroups and set hardcode resource limits 
2. Setup isloation for PID, mount, and networking namespaces.
3. Initialize job command in a new process
4. Run go routines to monitor job completion and failure and capture stdout/stderr output into a job log
5. Run job process until completion/failure
6. Run namespace and cgroup cleanup 

#### **Multi-Client Streaming**

There can multiple clients subscribed to subscribed a job's output. Each individual client simply has to create their own job reader through `GetNewLogReader` and call the `GetNextLine` functions to consume log lines at their own pace. 

### **3. gRPC Server**

#### **API Interface**

Here are the proposed proto definitions for the API:

```proto
service JobService {
    rpc Start(JobStartRequest) returns (JobStartResponse);
    rpc Stop(JobStopRequest) returns (JobStopResponse);
    rpc QueryStatus(JobQueryRequest) returns (JobInfo);
    rpc SubscribeOutput(JobSubscriptionRequest) returns (stream JobOutputResponse); // Updated for streaming
}

message JobStartRequest {
    string command = 1;
    repeated string args = 2;
}

message JobStartResponse {
    string jobId = 1;
}

message JobStopRequest {
    string jobId = 1;
}

message JobStopResponse {
    string status = 1; // e.g., "stopped", "not found", "error"
    string message = 2; // e.g., "Job successfully stopped", "Job not found", etc.
}

message JobQueryRequest {
    string jobId = 1;
}

message JobSubscriptionRequest {
    string jobId = 1;
}

message JobInfo {
    string jobId = 1;
    string status = 2; // e.g., "running", "stopped", "completed"
}

message JobOutputResponse {
    bytes stdout = 1;
    bytes stderr = 2;
}
```

#### **Functionality**

The gRPC will wrap the library and will similarly have methods for starting, stopping and querying job status/logs. To support streaming, it will continually read from a job's logs in a blocking fashing and stream data back to the client using gRPC streams.  

#### **TLS Setup**
We'll implement mTLS (Mutual TLS) for enhanced security.
* **Version:** TLS 1.3
* **Cipher Suites:** In Go, for TLS 1.3 the cypher suites are hard coded for simplicity and security. So we don't need to specify them.

#### Authorization Scheme 

The authorization scheme that we will have is that we will separate users into admins and viewers using organization unit parameter in certificates. Admins will be able to start, stop and query job state. Viewers will only be able to view job state.
