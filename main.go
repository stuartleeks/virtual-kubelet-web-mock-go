package main

import (
	"encoding/json"
	"net/http"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var pods map[string]*v1.Pod

func main() {
	pods = make(map[string]*v1.Pod)

	http.HandleFunc("/capacity", getCapacity)
	http.HandleFunc("/nodeAddresses", getNodeAddresses)
	http.HandleFunc("/nodeConditions", getNodeConditions)
	http.HandleFunc("/getPods", getPods)

	http.ListenAndServe(":3000", nil)
}
func getCapacity(w http.ResponseWriter, r *http.Request) {
	capacity := v1.ResourceList{
		"cpu":    resource.MustParse("20"),
		"memory": resource.MustParse("100Gi"),
		"pods":   resource.MustParse("20"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(capacity)
}
func getNodeAddresses(w http.ResponseWriter, r *http.Request) {
	nodeAddresses := []v1.NodeAddress{}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodeAddresses)
}
func getNodeConditions(w http.ResponseWriter, r *http.Request) {
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
	podList := []*v1.Pod{}
	for _, pod := range pods {
		podList = append(podList, pod)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(podList)
}
