package action

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

const kubeFolder = "/.kube/config"

type app struct {
	Name    string
	Version string
}

type AppFilters struct {
	Filters []string
}

func Run(filters AppFilters) error {

	printApps(connectCluster())
	return nil
}

func printApps(appsList *v1.PodList) {
	// Iterate over the apps, extracts the name, place them in a map for sorting
	mapApps := make(map[string]string)
	var app app

	for _, appsList := range appsList.Items {
		app.Version = strings.Split(appsList.Spec.Containers[0].Image, ":")[len(strings.Split(appsList.Spec.Containers[0].Image, ":"))-1]
		app.Name = appsList.Spec.Containers[0].Name

		mapApps[app.Name] = app.Version

	}

	//Formatting
	width := 5
	fmt.Printf("%-"+strconv.Itoa(width)+"s  %s\n", "APP", "VERSION")

	for key := range mapApps {
		//fmt.Println(mapApps[key])
		fmt.Printf("%-"+strconv.Itoa(width)+"s  %s\n", key, mapApps[key])
	}
}

func connectCluster() *v1.PodList {
	//Ensure to point to the user Home folder to fetch the .kube/config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	//Initiate communication with cluster
	var kubeConfig *kubernetes.Clientset = getClientSet(string(homeDir + kubeFolder))
	appsList, err := listApps(kubeConfig)
	if err != nil {
		log.Fatal(err)
	}
	return appsList
}

func listApps(clientSet kubernetes.Interface) (*v1.PodList, error) {
	pods, err := clientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return pods, nil
}

func getClientSet(kubeconfigPath string) *kubernetes.Clientset {
	//Generic Skaffolding for interfacing between go-client and kubernetes API
	var restConfig *rest.Config
	var err error

	if restConfig, err = rest.InClusterConfig(); err != nil {
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			klog.Fatal(err)
		}
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Fatal(err)
	}

	return clientSet
}
