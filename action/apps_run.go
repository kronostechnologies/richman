package action

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/go-yaml/yaml"
	"k8s.io/client-go/tools/clientcmd"
)

type AppsRun struct {
	Application string
	Config      map[string]string
}

type JobContext struct {
	Name      string
	Pod       string
	Container string
}

type JobYaml struct {
	Metadata struct {
		Name string
	}
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Name string
				}
			}
		}
	}
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

func (c *AppsRun) Run() error {
	//Create a new connection
	clientSet := &Connection{
		KubeConfigPath: GetKubeConfigPath(),
		Cluster:        GetClusterName(),
		ClientSet:      GetClientSet(GetKubeConfigPath()),
	}
	currentContext := clientSet.Cluster
	listApps, _ := ListApps(clientSet.ClientSet)
	mapApps := sortApps(listApps, currentContext)

	configMap, err := getConfigMap(c.Application, mapApps)
	if err != nil {
		fmt.Println("Config map not existing for this application")
		return err
	}

	//Sanitize the BS from byte[] to array of a yaml
	configMapStr := SanitizeConfigMap(configMap)

	jobTemplate := template.Must(template.New("configmap").Funcs(sprig.TxtFuncMap()).Parse(configMapStr))
	templateFields := ListTemplateFields(jobTemplate)

	currentApp := mapApps[c.Application]
	user, err := GetUser(currentApp.KubeContext.Cluster)
	if err != nil {
		return err
	}

	if name, ok := c.Config["name"]; ok {
		c.Config["name"] = fmt.Sprintf("%s-%s-%s", c.Application, user, name)
	} else {
		c.Config["name"] = fmt.Sprintf("%s-%s", c.Application, user)
	}

	if ve := validateConfig(templateFields, c.Config); ve != nil {
		return ve
	}

	buf := new(bytes.Buffer)
	if te := jobTemplate.Execute(buf, c.Config); te != nil {
		return te
	}

	tplConfig, te := ioutil.ReadAll(buf)
	if te != nil {
		return te
	}

	jobContext, pe := getJobContext(currentApp, tplConfig)
	jobConfigMap, ae := applyConfig(currentApp, tplConfig)
	if ae != nil {
		return ae
	}
	jobContext, pe = getJobContext(currentApp, jobConfigMap)
	if pe != nil {
		if ee, ok := pe.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "GetJobContext kubectl error: %s", ee.Stderr)
		}
	}

	channel := make(chan os.Signal)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-channel
		deleteJob(currentApp, jobContext)
		os.Exit(1)
	}()

	wpe := waitPod(currentApp, jobContext)

	deleteOnExit := false

	if wpe == nil {
		attach := true
		for attach {
			ace := attachContainer(currentApp, jobContext)

			time.Sleep(1 * time.Second)

			state, _ := getPodState(currentApp, jobContext)
			if strings.Contains(state, "Succeeded") || strings.Contains(state, "Failed") {
				deleteOnExit = true
				break
			}

			if ace != nil {
				fmt.Fprintln(os.Stderr, ace.Error())
			}

			c := promptChoice("kubectl has detached from " + state + " container. Attach, delete or quit (A/d/q)?")
			var option rune
			if len(c) == 0 {
				option = 'a'
			} else {
				option = []rune(strings.ToLower(c))[0]
			}

			switch option {
			case 'q':
				attach = false
			case 'd':
				attach = false
				deleteOnExit = true
			}
		}
	}

	if deleteOnExit {
		return deleteJob(currentApp, jobContext)
	}
	return nil
}

//Read the extracted configMap and apply it to the current cluster and namespace of the chosen app
func applyConfig(currentApp App, configMap []byte) ([]byte, error) {
	cmd := exec.Command("kubectl", "--context", currentApp.KubeContext.Cluster, "--namespace", currentApp.KubeContext.Namespace, "apply", "-f", "-", "-o", "yaml")
	cmd.Stdin = bytes.NewReader(configMap)
	out, err := cmd.Output()

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "applyConfig kubectl error: %s", ee.Stderr)
		}
	}

	return out, err
}

func getJobContext(currentApp App, jobYaml []byte) (*JobContext, error) {

	jobYamlStruct := &JobYaml{}
	ye := yaml.Unmarshal(jobYaml, &jobYamlStruct)
	if ye != nil {
		return nil, ye
	}
	if count := len(jobYamlStruct.Spec.Template.Spec.Containers); count != 1 {
		return nil, fmt.Errorf("container count %d unsupported", count)
	}

	jobName := jobYamlStruct.Metadata.Name
	containerName := jobYamlStruct.Spec.Template.Spec.Containers[0].Name

	cmd := exec.Command("kubectl", "--context", currentApp.KubeContext.Cluster, "--namespace", currentApp.KubeContext.Namespace, "get", "pod", "--selector=job-name="+jobName, "-o", "jsonpath={ .items[0].metadata.name }")
	out, ce := cmd.Output()
	podName := strings.TrimSpace(string(out))
	if ce != nil {
		return nil, ce
	}

	return &JobContext{
		Name:      jobName,
		Container: containerName,
		Pod:       podName,
	}, nil
}

func promptChoice(message string) string {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Fprintf(os.Stderr, "\n%s ", strings.TrimSpace(message))
	scanner.Scan()

	return strings.TrimSpace(scanner.Text())
}

func getPodState(currentApp App, jobCtx *JobContext) (string, error) {
	cmd := exec.Command("kubectl", "--context", currentApp.KubeContext.Cluster, "--namespace", currentApp.KubeContext.Namespace, "get", "pod", jobCtx.Pod, "-o", "jsonpath='{ .status.phase }'")
	out, ce := cmd.Output()
	return string(out), ce
}

func deleteJob(currentApp App, jobCtx *JobContext) error {
	cmd := exec.Command("kubectl", "--context", currentApp.KubeContext.Cluster, "--namespace", currentApp.KubeContext.Namespace, "delete", "job", jobCtx.Name)
	out, ce := cmd.Output()
	fmt.Println(string(out))

	if ce != nil {
		if ee, ok := ce.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "deleteJob kubectl error: %s", ee.Stderr)
		}
		return ce
	}

	return nil
}

func attachContainer(currentApp App, jobCtx *JobContext) error {
	cmd := exec.Command("kubectl", "--context", currentApp.KubeContext.Cluster, "--namespace", currentApp.KubeContext.Namespace, "attach", "-it", jobCtx.Pod, "-c", jobCtx.Container)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Use Ctrl-P,Ctrl-Q to detach from container")

	return cmd.Run()
}

func waitPod(currentApp App, jobContext *JobContext) error {
	tries := 1
	maxTries := 120

	for tries <= maxTries {
		out, ce := getPodState(currentApp, jobContext)

		if ce != nil {
			if ee, ok := ce.(*exec.ExitError); ok {
				fmt.Fprintf(os.Stderr, "ee kubectl error: %s", ee.Stderr)
			}
			return ce
		}

		if strings.Contains(out, "Running") || strings.Contains(out, "Failed") {
			break
		}

		fmt.Fprintf(os.Stderr, "\033[2K\rpod not ready: %s (%d/%d)", out, tries, maxTries)

		time.Sleep(5 * time.Second)
		tries += 1
	}
	
	fmt.Println()

	return nil
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

func formatUsername(username string) string {
	invalidCharactersRegex := regexp.MustCompile(`[^a-z0-9-]`)

	lowerUsername := strings.ToLower(username)

	return invalidCharactersRegex.ReplaceAllString(lowerUsername, "-")
}

func validateConfig(templateFields []TemplateField, config map[string]string) error {
	for _, v := range templateFields {
		if _, ok := config[v.Name]; ok == false {
			if v.Optional == false {
				return fmt.Errorf("required config '%s' missing", v.Name)
			} else {
				fmt.Printf("%s=\"%s\" (default value)\n", v.Name, v.Default)
			}
		} else {
			fmt.Printf("%s=\"%s\"\n", v.Name, config[v.Name])
		}
	}
	return nil
}

func SanitizeConfigMap(configMap []byte) string {
	configMapStr := strings.Replace(fmt.Sprintf("%s", configMap), `\n`, "\n", -1)
	configMapStr = strings.Replace(configMapStr, `\\\"`, "\"", -1)
	configMapStr = strings.Replace(configMapStr, `\`, "", -1)
	return configMapStr
}

//Get map of Apps, compares it with the app filter given by the -a flag for existence, and fetch the configmap if exists
func getConfigMap(application string, mapApps map[string]App) ([]byte, error) {
	cmdContext, _ := context.WithTimeout(context.Background(), 5*time.Second)

	//defer cancel()
	if _, ok := mapApps[application]; ok {
		application := mapApps[application].application
		namespace := mapApps[application].KubeContext.Namespace
		cluster := mapApps[application].KubeContext.Cluster
		configmap, err := exec.CommandContext(cmdContext, "kubectl", "--context", cluster, "--namespace", namespace, "get", "configmap", "-l", "richman/role=job-template,app.kubernetes.io/name="+application, "-o", "jsonpath={ .items[0].data.template }").Output()
		if err != nil {
			if ce := cmdContext.Err(); ce != nil {
				fmt.Fprintf(os.Stderr, "%s", ce)
			}
			if ee, ok := err.(*exec.ExitError); ok {
				fmt.Fprintf(os.Stderr, "%s", cluster)
				fmt.Fprintf(os.Stderr, "getConfigMap kubectl error: %s", ee.Stderr)
			}
			return nil, err
		}
		return configmap, err
	}
	return nil, nil
}
