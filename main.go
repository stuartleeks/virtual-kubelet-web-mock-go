package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var pods map[string]*v1.Pod

func main() {
	pods = make(map[string]*v1.Pod)

	// TODO - add filtering on HTTP Method type
	// TODO - error handling (checking missing query params, missing body, ...)
	http.HandleFunc("/capacity", getCapacity)
	http.HandleFunc("/nodeAddresses", getNodeAddresses)
	http.HandleFunc("/nodeConditions", getNodeConditions)
	http.HandleFunc("/getPods", getPods)
	http.HandleFunc("/getPodStatus", getPodStatus)
	http.HandleFunc("/createPod", createPod)
	http.HandleFunc("/updatePod", updatePod)
	http.HandleFunc("/deletePod", deletePod)

	http.ListenAndServe(":3000", nil)
}
func buildKeyFromNames(namespace string, name string) string {
	return fmt.Sprintf("%s-%s", namespace, name)
}
func addCorsHeaders(w *http.ResponseWriter, r *http.Request) bool {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	return r.Method == "OPTIONS"
}
func getCapacity(w http.ResponseWriter, r *http.Request) {
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
func getNodeAddresses(w http.ResponseWriter, r *http.Request) {
	log.Printf("getNodeAddresses")
	if addCorsHeaders(&w, r) {
		return
	}
	nodeAddresses := []v1.NodeAddress{}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodeAddresses)
}
func getNodeConditions(w http.ResponseWriter, r *http.Request) {
	log.Printf("getNodeConditions")
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

func getPods(w http.ResponseWriter, r *http.Request) {
	log.Printf("getPods - %s", r.Method)
	if addCorsHeaders(&w, r) {
		return
	}
	podList := []*v1.Pod{}
	for _, pod := range pods {
		podList = append(podList, pod)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(podList)
}

func getPodStatus(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")
	log.Printf("getPodStatus %s - %s", namespace, name)
	if addCorsHeaders(&w, r) {
		return
	}

	key := buildKeyFromNames(namespace, name)
	pod := pods[key]
	if pod == nil {
		log.Printf("getPodStatus. Pod not found: %s - %s", namespace, name)
		w.WriteHeader(404)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pod.Status)
}

func createPod(w http.ResponseWriter, r *http.Request) {
	if addCorsHeaders(&w, r) {
		return
	}
	var pod v1.Pod
	err := json.NewDecoder(r.Body).Decode(&pod)
	if err != nil {
		log.Printf("Error in createPod: %s", err)
		http.Error(w, err.Error(), 400)
	}
	log.Printf("createPod %s - %s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)

	pod.Status.Phase = v1.PodRunning
	pod.Status.Conditions = []v1.PodCondition{
		v1.PodCondition{
			Type:   v1.PodScheduled,
			Status: v1.ConditionTrue,
		},
		v1.PodCondition{
			Type:   v1.PodInitialized,
			Status: v1.ConditionTrue,
		},
		v1.PodCondition{
			Type:   v1.PodReady,
			Status: v1.ConditionTrue,
		},
	}

	now := metav1.NewTime(time.Now())

	for _, container := range pod.Spec.Containers {
		status := v1.ContainerStatus{
			Name:         container.Name,
			Image:        container.Image,
			Ready:        true,
			RestartCount: 0,
			State: v1.ContainerState{
				Running: &v1.ContainerStateRunning{
					StartedAt: now,
				},
			},
		}
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, status)
	}

	key := buildKeyFromNames(pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
	pods[key] = &pod
}
func updatePod(w http.ResponseWriter, r *http.Request) {
	if addCorsHeaders(&w, r) {
		return
	}
	var pod v1.Pod
	err := json.NewDecoder(r.Body).Decode(&pod)
	if err != nil {
		log.Printf("Error in updatePod: %s", err)
		http.Error(w, err.Error(), 400)
	}
	log.Printf("updatePod %s - %s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)

	key := buildKeyFromNames(pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
	pods[key] = &pod
}
func deletePod(w http.ResponseWriter, r *http.Request) {
	if addCorsHeaders(&w, r) {
		return
	}
	var pod v1.Pod
	err := json.NewDecoder(r.Body).Decode(&pod)
	if err != nil {
		log.Printf("Error in deletePod: %s", err)
		http.Error(w, err.Error(), 400)
	}
	log.Printf("deletePod %s - %s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)

	key := buildKeyFromNames(pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
	delete(pods, key)
}
