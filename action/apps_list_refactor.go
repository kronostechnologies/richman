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

type App struct {
	name       string
	version    string
	containers []string
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
	mapApps := make(map[string]App)
	listContainers := make([]string, 10)
	for _, appsList := range appsList.Items {
		//All containers in a pod are put into a slice
		for i, container := range appsList.Spec.Containers {
			listContainers[i] = container.Name
		}

		//After each iteration, we have an App struct which contain the name, Version, and all the containers
		// names attached to a pod
		mapApps[appsList.Spec.Containers[0].Name] = App{
			name:       appsList.Spec.Containers[0].Name,
			version:    strings.Split(appsList.Spec.Containers[0].Image, ":")[len(strings.Split(appsList.Spec.Containers[0].Image, ":"))-1],
			containers: listContainers,
		}

	}

	//Formatting
	width := 5
	fmt.Printf("%-"+strconv.Itoa(width)+"s  %s\n", "APP", "VERSION")

	for key := range mapApps {
		fmt.Printf("%-"+strconv.Itoa(width)+"s  %s\n", key, mapApps[key].version)
		fmt.Println("======================================")
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
