package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/zeebe-io/zeebe/clients/go/pkg/entities"
	"github.com/zeebe-io/zeebe/clients/go/pkg/pb"
	"github.com/zeebe-io/zeebe/clients/go/pkg/worker"
	"github.com/zeebe-io/zeebe/clients/go/pkg/zbc"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func main() {
	zbClient := getClient()
	getStatus(zbClient)
	deploy(zbClient)

	go zbClient.NewJobWorker().JobType("get-time").Handler(handleGetTime).Open()

	http.HandleFunc("/start", createStartHandler(zbClient))
	http.ListenAndServe(":3000", nil)
}

type Time struct {
	Time   string `json:"time"`
	Hour   int    `json:"hour"`
	Minute int    `json:"minute"`
	Second int    `json:"second"`
	Day    int    `json:"day"`
	Month  int    `json:"month"`
	Year   int    `json:"year"`
}

type GetTimeCompleteVariables struct {
	Time Time `json:"time"`
}

func handleGetTime(client worker.JobClient, job entities.Job) {
	//log.Println(job)
	//ctx := context.Background()
	//client.NewCompleteJobCommand().JobKey(job.Key).Send(ctx)

	var (
		data []byte
	)

	response, err := http.Get("https://json-api.joshwulf.com/time")
	if err != nil {
		fmt.Printf("The HTTP request failed with error %s\n", err)
		return
	} else {
		data, _ = ioutil.ReadAll(response.Body)
	}

	var time Time
	err = json.Unmarshal(data, &time)
	if err != nil {
		log.Fatalln(err)
		return
	}

	payload := &GetTimeCompleteVariables{time}

	ctx := context.Background()
	cmd, _ := client.NewCompleteJobCommand().JobKey(job.Key).VariablesFromObject(payload)
	_, err = cmd.Send(ctx)

	if err != nil {
		log.Fatalln(err)
	}
}

func getClient() zbc.Client {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	gatewayAddress := os.Getenv("ZEEBE_ADDRESS")

	zbClient, err := zbc.NewClient(&zbc.ClientConfig{
		GatewayAddress: gatewayAddress,
	})

	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	return zbClient
}

func getStatus(zbClient zbc.Client) {
	ctx := context.Background()
	topology, err := zbClient.NewTopologyCommand().Send(ctx)
	if err != nil {
		panic(err)
	}

	for _, broker := range topology.Brokers {
		fmt.Println("Broker", broker.Host, ":", broker.Port)
		for _, partition := range broker.Partitions {
			fmt.Println("  Partition", partition.PartitionId, ":", roleToString(partition.Role))
		}
	}
}

func deploy(zbClient zbc.Client) {
	ctx := context.Background()
	response, err := zbClient.NewDeployWorkflowCommand().AddResourceFile("test-process.bpmn").Send(ctx)
	if err != nil {
		panic(err)
	}
	log.Println(response.String())
}

type BoundHandler func(w http.ResponseWriter, r *http.Request)

func createStartHandler(client zbc.Client) BoundHandler {
	f := func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		request, err := client.NewCreateInstanceCommand().BPMNProcessId("test-process").LatestVersion().WithResult().Send(ctx)
		if err != nil {
			panic(err)
		}
		fmt.Fprint(w, request.String())
	}
	return f
}

func roleToString(role pb.Partition_PartitionBrokerRole) string {
	switch role {
	case pb.Partition_LEADER:
		return "Leader"
	case pb.Partition_FOLLOWER:
		return "Follower"
	default:
		return "Unknown"
	}
}
