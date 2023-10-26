package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	pb "job_worker_service/pkg/api/proto"

	"job_worker_service/pkg/utils"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var rootCmd = &cobra.Command{Use: "cli"}

var serverAddr string

func init() {
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "localhost:50051", "gRPC server address")
	logCmd.Flags().BoolP("follow", "f", false, "Follow the job's output stream")
	rootCmd.AddCommand(startCmd, stopCmd, statusCmd, logCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func getClient() pb.JobServiceClient {
	// Load the client's certificate and private key
	cert, err := tls.LoadX509KeyPair(utils.GetRelativePath("keys/admin.crt"), utils.GetRelativePath("keys/admin.key"))
	if err != nil {
		log.Fatalf("failed to load client cert/key: %v", err)
	}

	// Create a certificate pool
	certPool := x509.NewCertPool()

	// Load the CA certificate that was used to sign the server's certificate
	ca, err := os.ReadFile(utils.GetRelativePath("keys/ca.crt"))
	if err != nil {
		log.Fatalf("could not read CA certificate: %v", err)
	}

	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		log.Fatalf("failed to append server certs")
	}

	// Create the TLS credentials for the client
	creds := credentials.NewTLS(&tls.Config{
		ServerName:   "", // You can leave this empty if not using server name verification
		Certificates: []tls.Certificate{cert},
		RootCAs:      certPool,
	})

	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}
	return pb.NewJobServiceClient(conn)
}

var startCmd = &cobra.Command{
	Use:   "start [command] [args...]",
	Short: "Start a job",
	Example: `
# Start a job with the "ls" command
cli start ls -l`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		resp, err := client.Start(context.Background(), &pb.JobStartRequest{Command: args[0], Args: args[1:]})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Started job with ID:", resp.GetJobId())
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop [jobID]",
	Short: "Stop a job",
	Example: `
# Stop a job with ID "12345"
cli stop 12345`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		resp, err := client.Stop(context.Background(), &pb.JobStopRequest{JobId: args[0]})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Job status:", resp.GetStatus(), "Message:", resp.GetMessage())
	},
}

var statusCmd = &cobra.Command{
	Use:   "status [jobID]",
	Short: "Query the status of a job",
	Example: `
# Query status of a job with ID "12345"
cli status 12345`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		resp, err := client.QueryStatus(context.Background(), &pb.JobQueryRequest{JobId: args[0]})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Job ID:", resp.GetJobId(), "Status:", resp.GetStatus())
	},
}

var logCmd = &cobra.Command{
	Use:   "log [jobID]",
	Short: "Subscribe to a job's output",
	Example: `
# Subscribe to the output of a job with ID "12345"
cli log 12345`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		id := args[0]
		follow := cmd.Flag("follow").Value.String() == "true"
		if follow {
			stream, err := client.SubscribeOutput(context.Background(), &pb.JobSubscriptionRequest{JobId: id})
			if err != nil {
				log.Fatal(err)
			}

			// Continuously read from the stream if the follow flag is set
			for {
				resp, err := stream.Recv()
				if err != nil {
					break
				}
				fmt.Print(string(resp.GetOutput()))
			}
		} else {
			resp, err := client.GetOutput(context.Background(), &pb.JobQueryRequest{JobId: id})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Print(string(resp.GetOutput()))
		}
	},
}
