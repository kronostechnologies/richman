package action

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"os/exec"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const kubeFolder = "/.kube/config"

type KubeContext struct {
	Namespace   string
	Cluster     string
	Application string
}

type Connection struct {
	KubeConfigPath string
	ClientSet      *kubernetes.Clientset
	Cluster        string
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

func (clusterConnection Connection) NewConnection() bool {
	conn := ConnectCluster()
	if conn == nil {
		fmt.Println("Impossible to establish a communication with your cluster at this time")
		return false
	}
	clientCfg, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	fmt.Printf("%-v %s", clientCfg, err)
	fmt.Println("------------")
	return true
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

func GetUser(cluster string) (string, error) {
	findKubeUserRegex := regexp.MustCompile(`:([^:@]+)@`)

	out, err := exec.Command("kubectl", "config", "view", "-o", "jsonpath={ .contexts[?(@.name == \""+cluster+"\")].context.user }").Output()
	if err != nil {
		return "", err
	}
	matches := findKubeUserRegex.FindStringSubmatch(string(out))
	var username string
	if len(matches) == 2 && matches[1] != "" {
		username = matches[1]
	} else {
		username = os.Getenv("USER")
	}

	return formatUsername(username), nil
}
