package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	tasks "google.golang.org/genproto/googleapis/cloud/tasks/v2"
)

const (
	locationID = "us-central1"
	queueID    = "default"
)

func createTask(ctx context.Context, handler_path string, payload []byte) (*tasks.Task, error) {
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("NewClient: %v", err)
	}

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", projectID, locationID, queueID)

	req := &tasks.CreateTaskRequest{
		Parent: queuePath,
		Task: &tasks.Task{
			MessageType: &tasks.Task_AppEngineHttpRequest{
				AppEngineHttpRequest: &tasks.AppEngineHttpRequest{
					HttpMethod:  tasks.HttpMethod_POST,
					RelativeUri: handler_path,
					Body:        payload,
				},
			},
		},
	}

	createdTask, err := client.CreateTask(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("cloudtasks.CreateTask: %v", err)
	}

	return createdTask, nil
}

//processes the payload received from G's webhook delivery system
func processHookLater(ctx context.Context, data []byte) {
	if _, err := createTask(ctx, "/task/process_hook", data); err != nil {
		log.Printf("trouble scheduling task %v", err)
	}
}

//fetches and stores one single webhook
func saveResourceLater(ctx context.Context, hook *hookStruct) {
	body, err := json.Marshal(hook)
	if err != nil {
		log.Printf("trouble encoding %v -> %v", hook, err)
		return
	}
	if _, err := createTask(ctx, "/task/save_resource", body); err != nil {
		log.Printf("trouble scheduling task %v", err)
	}

}

//purgeBeforeLate is expecting time stamp anything older than stamp will be scheduled for deletion
func purgeBeforeLater(ctx context.Context, t time.Time) {
	body, err := json.Marshal(t)
	if err != nil {
		log.Printf("trouble encoding %v -> %v", t, err)
		return
	}
	if _, err := createTask(ctx, "/task/purge_before", body); err != nil {
		log.Printf("trouble scheduling task %v", err)
	}
}

//purgeStepLater is expecting query cursor to continue purging
func purgeStepLater(ctx context.Context, msg string) {
	if _, err := createTask(ctx, "/task/purge_step", []byte(msg)); err != nil {
		log.Printf("trouble scheduling task %v", err)
	}
}
