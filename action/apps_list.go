package action

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const kubeFolder = "/.kube/config"

type App struct {
	name        string
	application string
	version     string
	containers  []v1.Container
	labels      map[string]string
	KubeContext KubeContext
}

type KubeContext struct {
	Namespace   string
	Cluster     string
	Application string
}

type AppFilters struct {
	Filters []string
}

type Connection struct {
	KubeConfigPath string
	ClientSet      *kubernetes.Clientset
	Cluster        string
}

func Run(filters AppFilters) error {

	//Create a new connection
	clientSet := &Connection{
		KubeConfigPath: GetKubeConfigPath(),
		Cluster:        GetClusterName(),
		ClientSet:      GetClientSet(GetKubeConfigPath()),
	}
	currentContext := clientSet.Cluster
	listApps, _ := ListApps(clientSet.ClientSet)
	mapApps := sortApps(listApps, currentContext)

	//Actual listing
	PrintApps(mapApps)
	return nil
}

func sortApps(appsList *v1.PodList, context string) map[string]App {
	// Iterate over the apps, extracts the name, place them in a map for sorting
	mapApps := make(map[string]App)

	//After each iteration, we have an App struct which contain the name, Version, and all the containers
	// names attached to a pod. A kubeContext struct with the namespace, the cluster and the app name is tagged along.
	for _, app := range appsList.Items { //If the app already exist, we simply append the containers in its entry
		appName := GetLabels(app.ObjectMeta)["app.kubernetes.io/name"]
		if cont, ok := mapApps[appName]; ok {
			if len(app.ObjectMeta.Labels["app.kubernetes.io/name"]) > 0 && app.ObjectMeta.Labels["app.kubernetes.io/component"] != mapApps[appName].labels["app.kubernetes.io/component"] {
				mapApps[GetLabels(app.ObjectMeta)["app.kubernetes.io/name"]] = cont
			}
		} else {
			mapApps[appName] = App{
				name:        app.Spec.Containers[0].Name,
				labels:      GetLabels(app.ObjectMeta),
				application: GetLabels(app.ObjectMeta)["app.kubernetes.io/name"],
				version:     strings.Split(app.Spec.Containers[0].Image, ":")[len(strings.Split(app.Spec.Containers[0].Image, ":"))-1],
				containers:  app.Spec.Containers,
				KubeContext: KubeContext{
					Namespace: app.Namespace,
					Cluster:   context,
				},
			}
		}
	}
	return mapApps
}
func GetLabels(metadata metav1.ObjectMeta) map[string]string {

	if metadata.Labels != nil {
		mapLabels := make(map[string]string)
		for key, label := range metadata.Labels {
			mapLabels[key] = label
		}
		return mapLabels
	}
	return nil
}

//Print Labels for each pods
func PrintLabels(mapApps map[string]App) {
	//Formatting
	width := "5"
	fmt.Printf("%-"+width+"s  %s\n", "Label", "Value")

	for key := range mapApps {
		fmt.Printf("%-"+width+"s  %s\n", key, mapApps[key].version)
		fmt.Println("---------------------")
		for k := range mapApps[key].labels {
			fmt.Printf("name : %s Label: %s", k, mapApps[key].labels[k])
		}
	}
}

func PrintApps(mapApps map[string]App) {

	//Formatting
	width := "5"

	fmt.Printf("%-"+width+"s  %s\n", "APP", "VERSION")
	fmt.Println("======================================")
	for key := range mapApps {
		for _, container := range mapApps[key].containers {
			fmt.Printf("%s : %s %s \n", key, container.Name, strings.Split(container.Image, ":")[len(strings.Split(container.Image, ":"))-1])
		}
		fmt.Printf("-------------- \n")
	}
}

//Initiate communication with cluster
func ConnectCluster() *kubernetes.Clientset {
	//Ensure to point to the user Home folder to fetch the .kube/config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("impossible to read your kube config file, ensure ~/kube/config is available")
		return nil
	}
	return GetClientSet(string(homeDir + kubeFolder))
}

//Return a pointer to a list of Pods
func ListApps(clientSet kubernetes.Interface) (*v1.PodList, error) {
	pods, err := clientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return pods, nil
}

//~/.kube/config
func GetKubeConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ("")
	}
	return string(homeDir + kubeFolder)
}

//kubectl config --current-context
func GetClusterName() string {
	cmd := exec.Command("kubectl", "config", "current-context")
	context, err := cmd.Output()
	if err != nil {
		log.Fatal("Impossible to fetch the name of your cluster, is your kube config reachable?\n", err)
	}
	return TrimClusterName(context)
}

//Remove the line feed on the cluster Name
func TrimClusterName(cluster []byte) string {
	if len(cluster) == 0 {
		log.Fatal("Impossible to fetch any cluster name, is your kubeConfig properly configured?\n")
	}
	return string(cluster[:len(cluster)-1])
}

func GetClientSet(kubeconfigPath string) *kubernetes.Clientset {
	//Generic Skaffolding for interfacing between go-client and kubernetes API
	var restConfig *rest.Config
	var err error

	if restConfig, err = rest.InClusterConfig(); err != nil {
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			log.Fatal(err)
		}
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatal(err)
	}

	return clientSet
}
