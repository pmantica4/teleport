package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"sync"

	pb "job_worker_service/pkg/api/proto"
	job "job_worker_service/pkg/job_manager"
	"job_worker_service/pkg/utils"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ----- JobStore functionality -----

type JobStore struct {
	jobs      map[string]*job.Job
	jobsMutex sync.RWMutex
}

var jobStore = &JobStore{
	jobs: make(map[string]*job.Job),
}

func StartJob(command string, args ...string) (string, error) {
	j, err := job.StartJob(command, args...)
	if err != nil {
		return "", err
	}
	jobStore.jobsMutex.Lock()
	id := uuid.New().String()
	jobStore.jobs[fmt.Sprint(id)] = j
	jobStore.jobsMutex.Unlock()
	return id, nil
}

func GetJob(jobID string) (*job.Job, error) {
	jobStore.jobsMutex.RLock()
	defer jobStore.jobsMutex.RUnlock()

	j, exists := jobStore.jobs[jobID]
	if !exists {
		return nil, errors.New("job not found")
	}
	return j, nil
}

// ----- Server functionality -----

type server struct {
	pb.UnimplementedJobServiceServer
}

func (s *server) Start(ctx context.Context, req *pb.JobStartRequest) (*pb.JobStartResponse, error) {
	id, err := StartJob(req.GetCommand(), req.GetArgs()...)
	if err != nil {
		return nil, err
	}
	return &pb.JobStartResponse{JobId: id}, nil
}

// Stop a job
func (s *server) Stop(ctx context.Context, req *pb.JobStopRequest) (*pb.JobStopResponse, error) {
	job, err := GetJob(req.GetJobId())
	if err != nil {
		return &pb.JobStopResponse{Status: "error", Message: err.Error()}, nil
	}
	err = job.Stop()
	if err != nil {
		return &pb.JobStopResponse{Status: "error", Message: err.Error()}, nil
	}
	return &pb.JobStopResponse{Status: "stopped", Message: "Job successfully stopped"}, nil
}

// Query the status of a job
func (s *server) QueryStatus(ctx context.Context, req *pb.JobQueryRequest) (*pb.JobInfo, error) {
	job, err := GetJob(req.GetJobId())
	if err != nil {
		return nil, err
	}
	status := job.GetStatus()
	return &pb.JobInfo{JobId: req.GetJobId(), Status: string(status)}, nil
}

// Get the current output of a job
func (s *server) GetOutput(ctx context.Context, req *pb.JobQueryRequest) (*pb.JobOutputResponse, error) {
	job, err := GetJob(req.GetJobId())
	if err != nil {
		return nil, err
	}
	lines := job.ReadAllLines()

	// Flatten lines into bytes
	output := []byte{}
	for _, line := range lines {
		output = append(output, []byte(line)...)
	}
	return &pb.JobOutputResponse{Output: output}, nil
}

// Subscribe to the output of a job
func (s *server) SubscribeOutput(req *pb.JobSubscriptionRequest, stream pb.JobService_SubscribeOutputServer) error {
	job, err := GetJob(req.GetJobId())
	if err != nil {
		return err
	}
	logReader := job.NewLogReader()
	for {
		output, _ := logReader.ReadNextLine(true)
		if err := stream.Send(&pb.JobOutputResponse{Output: []byte(output)}); err != nil {
			return err
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair(utils.GetRelativePath("keys/server.crt"), utils.GetRelativePath("keys/server.key"))
	if err != nil {
		log.Fatalf("failed to load server cert/key: %v", err)
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(utils.GetRelativePath("keys/ca.crt"))
	if err != nil {
		log.Fatalf("failed to read ca certificate: %v", err)
	}

	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		log.Fatalf("failed to append client certs")
	}

	// Create the TLS credentials
	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS13,
	})

	// Create the gRPC server with the credentials
	s := grpc.NewServer(grpc.Creds(creds))

	pb.RegisterJobServiceServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
