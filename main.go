package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerfilters "github.com/docker/docker/api/types/filters"
	dockernetwork "github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type webProvider struct {
	dockerClient *dockerclient.Client
}

func main() {

	dockerCli, err := dockerclient.NewEnvClient()
	if err != nil {
		log.Panicf("Error creating docker client: %s", err)
	}
	provider := webProvider{
		dockerClient: dockerCli,
	}
	provider.initializeHandlers()
	http.ListenAndServe(":3000", nil)
}

func createDockerContainerName(namespace string, podName string, containerName string) string {
	return fmt.Sprintf("VK_%s_%s_%s", namespace, podName, containerName)
}
func addCorsHeaders(w *http.ResponseWriter, r *http.Request) bool {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	return r.Method == "OPTIONS"
}
func logHTTPError(w http.ResponseWriter, code int, format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	log.Printf(message)
	http.Error(w, message, code)
}
func (p *webProvider) initializeHandlers() {
	// TODO - add filtering on HTTP Method type
	// TODO - error handling (checking missing query params, missing body, ...)
	http.HandleFunc("/capacity", p.getCapacity)
	http.HandleFunc("/nodeAddresses", p.getNodeAddresses)
	http.HandleFunc("/nodeConditions", p.getNodeConditions)
	http.HandleFunc("/getPods", p.getPods)
	http.HandleFunc("/getPodStatus", p.getPodStatus)
	http.HandleFunc("/createPod", p.createPod)
	// http.HandleFunc("/updatePod", updatePod)
	http.HandleFunc("/deletePod", p.deletePod)
	http.HandleFunc("/getContainerLogs", p.getContainerLogs)
}
func (p *webProvider) getCapacity(w http.ResponseWriter, r *http.Request) {
	log.Printf("getCapacity")
	if addCorsHeaders(&w, r) {
		return
	}
	capacity := v1.ResourceList{
		"cpu":    resource.MustParse("20"),
		"memory": resource.MustParse("100Gi"),
		"pods":   resource.MustParse("20"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(capacity)
}
func (p *webProvider) getNodeAddresses(w http.ResponseWriter, r *http.Request) {
	// log.Printf("getNodeAddresses")
	if addCorsHeaders(&w, r) {
		return
	}
	nodeAddresses := []v1.NodeAddress{}
	kubeletPodIP := os.Getenv("VKUBELET_POD_IP")
	if kubeletPodIP != "" {
		nodeAddress := v1.NodeAddress{
			Address: kubeletPodIP,
			Type:    v1.NodeInternalIP,
		}
		nodeAddresses = append(nodeAddresses, nodeAddress)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodeAddresses)
}
func (p *webProvider) getNodeConditions(w http.ResponseWriter, r *http.Request) {
	// log.Printf("getNodeConditions")
	if addCorsHeaders(&w, r) {
		return
	}
	nodeConditions := []v1.NodeCondition{
		{
			Type:               "Ready",
			Status:             v1.ConditionTrue,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletReady",
			Message:            "At your service",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodeConditions)
}

func (p *webProvider) getPodsFromDocker() ([]*v1.Pod, error) {

	podList := []*v1.Pod{}
	filters := dockerfilters.NewArgs()
	filters.Add("name", "VK_*")
	containerList, err := p.dockerClient.ContainerList(context.Background(), dockertypes.ContainerListOptions{
		Filters: filters,
	},
	)
	if err != nil {
		return nil, fmt.Errorf("ContainerList failed: %v", err)
	}
	now := metav1.NewTime(time.Now())
	for _, container := range containerList {
		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: container.Labels["podNamespace"],
				Name:      container.Labels["podName"],
				Labels: map[string]string{
					"containerID": container.ID,
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					v1.Container{
						Image: container.Image,
					},
				},
			},
			// TODO - Fixed for now - convert these from the container state
			Status: v1.PodStatus{
				Phase:     v1.PodRunning,
				Message:   "Running",
				StartTime: &now,
				Conditions: []v1.PodCondition{
					{
						Type:   v1.PodInitialized,
						Status: v1.ConditionTrue,
					},
					{
						Type:   v1.PodReady,
						Status: v1.ConditionTrue,
					},
					{
						Type:   v1.PodScheduled,
						Status: v1.ConditionTrue,
					},
				},
				ContainerStatuses: []v1.ContainerStatus{
					v1.ContainerStatus{
						Name:         container.Names[0],
						Image:        container.Image,
						Ready:        true,
						RestartCount: 0,
						State: v1.ContainerState{
							Running: &v1.ContainerStateRunning{
								StartedAt: now,
							},
						},
					},
				},
			},
		}
		podList = append(podList, &pod)
	}
	return podList, nil
}
func (p *webProvider) getPods(w http.ResponseWriter, r *http.Request) {
	log.Printf("getPods")

	if addCorsHeaders(&w, r) {
		return
	}

	podList, err := p.getPodsFromDocker()
	if err != nil {
		logHTTPError(w, 400, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(podList)
}

func (p *webProvider) getPodStatus(w http.ResponseWriter, r *http.Request) {

	// TODO - check status of the container!

	namespace := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")
	log.Printf("getPodStatus %s - %s", namespace, name)
	if addCorsHeaders(&w, r) {
		return
	}

	podList, err := p.getPodsFromDocker()
	if err != nil {
		logHTTPError(w, 400, err.Error())
		return
	}

	for _, pod := range podList {
		if pod.ObjectMeta.Namespace == namespace && pod.ObjectMeta.Name == name {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(pod.Status)
			return
		}
	}
	log.Printf("getPodStatus. Pod not found: %s - %s", namespace, name)
	w.WriteHeader(404)
}

func (p *webProvider) createPod(w http.ResponseWriter, r *http.Request) {
	if addCorsHeaders(&w, r) {
		return
	}
	var pod v1.Pod
	err := json.NewDecoder(r.Body).Decode(&pod)
	if err != nil {
		logHTTPError(w, 400, "Error in createPod: %s", err)
		return
	}
	log.Printf("createPod %s - %s - %s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)

	// TODO Currently only handling a single container, for simplicity
	if len(pod.Spec.Containers) != 1 {
		http.Error(w, "CreatePod currently only supports a single container per pod", 400)
		return
	}
	containerSpec := pod.Spec.Containers[0]

	config := dockercontainer.Config{
		Image: containerSpec.Image,
		Labels: map[string]string{
			"podNamespace": pod.ObjectMeta.Namespace,
			"podName":      pod.ObjectMeta.Name,
		},
	}
	containerName := createDockerContainerName(pod.ObjectMeta.Namespace, pod.ObjectMeta.Name, containerSpec.Name)

	log.Printf("Pulling image %s\n", containerSpec.Image)
	readerCloser, err := p.dockerClient.ImagePull(context.Background(), containerSpec.Image, dockertypes.ImagePullOptions{})
	if err != nil {
		logHTTPError(w, 400, "ImagePull failed: %v", err)
		return
	}
	buf := make([]byte, 64)
	for ok := true; ok; ok = true {
		n, err := readerCloser.Read(buf)
		if err != nil {
			break
		}
		log.Printf("%s", buf[:n])
	}
	log.Printf("Pulled image %s\n", containerSpec.Image)

	pod.Status.Phase = v1.PodPending
	pod.Status.Message = "Creating"
	// TODO - add to pod list so that status is reported...
	log.Printf("Creating container %s\n", containerName)
	// TODO handle exposing ports
	container, err := p.dockerClient.ContainerCreate(context.Background(), &config, &dockercontainer.HostConfig{}, &dockernetwork.NetworkingConfig{}, containerName)
	if err != nil {
		logHTTPError(w, 400, "ContainerCreate failed: %v", err)
		return
	}
	log.Printf("Created container %s. ID: %s\n", containerName, container.ID)

	pod.Status.Message = "Starting"
	log.Printf("Starting container %s\n", container.ID)
	err = p.dockerClient.ContainerStart(context.Background(), container.ID, dockertypes.ContainerStartOptions{})
	if err != nil {
		logHTTPError(w, 400, "ContainerStart failed: %v", err)
		return
	}
	// TODO does ContainerStart wait for container to start before returning?
	log.Printf("Started container %s\n", container.ID)
}

func (p *webProvider) deletePod(w http.ResponseWriter, r *http.Request) {
	if addCorsHeaders(&w, r) {
		return
	}
	var podToDelete v1.Pod
	err := json.NewDecoder(r.Body).Decode(&podToDelete)
	if err != nil {
		logHTTPError(w, 400, "Error in deletePod: %s", err)
		return
	}
	log.Printf("deletePod %s - %s - %s", podToDelete.Namespace, podToDelete.ObjectMeta.Namespace, podToDelete.ObjectMeta.Name)

	podList, err := p.getPodsFromDocker()
	if err != nil {
		logHTTPError(w, 400, err.Error())
		return
	}

	namespace := podToDelete.ObjectMeta.Namespace
	name := podToDelete.ObjectMeta.Name

	for _, pod := range podList {
		if pod.ObjectMeta.Namespace == namespace && pod.ObjectMeta.Name == name {
			containerID := pod.ObjectMeta.Labels["containerID"]
			err = p.dockerClient.ContainerRemove(context.Background(), containerID, dockertypes.ContainerRemoveOptions{Force: true})
			if err != nil {
				logHTTPError(w, 400, "ContainerRemove failed: %v", err)
				return
			}
			return
		}
	}
	log.Printf("getPodStatus. Pod not found: %s - %s", namespace, name)
	w.WriteHeader(404)

}
func (p *webProvider) getContainerLogs(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	podName := r.URL.Query().Get("podName")
	containerName := r.URL.Query().Get("containerName")
	log.Printf("getContainerLogs %s - %s - %s", namespace, podName, containerName)
	if addCorsHeaders(&w, r) {
		return
	}

	dockerContainerName := createDockerContainerName(namespace, podName, containerName)

	readerCloser, err := p.dockerClient.ContainerLogs(context.Background(), dockerContainerName, dockertypes.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		logHTTPError(w, 400, "ContainerLogs failed: %s", err)
		return
	}
	defer readerCloser.Close()
	io.Copy(w, readerCloser)
}
