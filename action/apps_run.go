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
	configMap, err := getConfigMap(c.Application, currentContext)
	if err != nil {
		fmt.Println("Config map not existing for this application")
		return err
	}

	//Sanitize the BS from byte[] to array of a yaml
	configMapStr := SanitizeConfigMap(configMap)

	jobTemplate := template.Must(template.New("configmap").Funcs(sprig.TxtFuncMap()).Parse(configMapStr))
	templateFields := ListTemplateFields(jobTemplate)

	user, err := GetUser(currentContext)
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

	fmt.Println("ready to execute job")
	buf := new(bytes.Buffer)
	if te := jobTemplate.Execute(buf, c.Config); te != nil {
		return te
	}

	tplConfig, te := ioutil.ReadAll(buf)
	if te != nil {
		return te
	}

	jobName, je := getJobName(tplConfig)
	if je != nil {
		return je
	}
	deleteJobIfComplete(currentContext, c.Application, jobName)

	jobContext, pe := getJobContext(currentContext, c.Application, tplConfig)
	jobConfigMap, ae := applyConfig(currentContext, c.Application, tplConfig)
	if ae != nil {
		// Ask to user if he wants to delete the job
		cs := promptChoice("kubectl has failed to replace the existing job. Do you want to delete it (y/[n])?")
		if len(cs) == 0 {
			cs = "n"
		}
		if []rune(strings.ToLower(cs))[0] == 'y' {
			de := deleteJob(currentContext, c.Application, jobName)
			if de != nil {
				if ee, ok := de.(*exec.ExitError); ok {
					fmt.Fprintf(os.Stderr, "deleteJob kubectl error: %s", ee.Stderr)
				}
				return de
			}
		} else {
			return ae
		}

		jobConfigMap, ae = applyConfig(currentContext, c.Application, tplConfig)
		if ae != nil {
			return ae
		}
	}
	jobContext, pe = getJobContext(currentContext, c.Application, jobConfigMap)
	if pe != nil {
		if ee, ok := pe.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "GetJobContext kubectl error: %s", ee.Stderr)
		}
	}

	channel := make(chan os.Signal)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-channel
		deleteJob(currentContext, c.Application, jobName)
		os.Exit(1)
	}()

	wpe := waitPod(currentContext, c.Application, jobContext)
	deleteOnExit := false

	if wpe == nil {
		attach := true
		for attach {
			ace := attachContainer(currentContext, c.Application, jobContext)

			time.Sleep(1 * time.Second)

			state, _ := getPodState(currentContext, c.Application, jobContext)
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
		return deleteJob(currentContext, c.Application, jobName)
	}
	return nil
}

// Read the extracted configMap and apply it to the current cluster and namespace of the chosen app
func applyConfig(cluster string, application string, configMap []byte) ([]byte, error) {
	cmd := exec.Command("kubectl", "--context", cluster, "--namespace", application, "apply", "-f", "-", "-o", "yaml")
	cmd.Stdin = bytes.NewReader(configMap)
	out, err := cmd.Output()

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "applyConfig kubectl error: %s", ee.Stderr)
		}
	}

	return out, err
}

func getJobName(jobYaml []byte) (string, error) {
	jobYamlStruct := &JobYaml{}
	ye := yaml.Unmarshal(jobYaml, &jobYamlStruct)
	if ye != nil {
		return "", ye
	}
	if count := len(jobYamlStruct.Spec.Template.Spec.Containers); count != 1 {
		return "", fmt.Errorf("container count %d unsupported", count)
	}
	return jobYamlStruct.Metadata.Name, nil
}

func getJobContext(cluster string, application string, jobYaml []byte) (*JobContext, error) {

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

	cmd := exec.Command("kubectl", "--context", cluster, "--namespace", application, "get", "pod", "--selector=job-name="+jobName, "-o", "jsonpath={ .items[0].metadata.name }")
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

func getPodState(cluster string, application string, jobCtx *JobContext) (string, error) {
	cmd := exec.Command("kubectl", "--context", cluster, "--namespace", application, "get", "pod", jobCtx.Pod, "-o", "jsonpath='{ .status.phase }'")
	out, ce := cmd.Output()
	return string(out), ce
}

func deleteJob(cluster string, application string, jobName string) error {
	cmd := exec.Command("kubectl", "--context", cluster, "--namespace", application, "delete", "job", jobName)
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

func deleteJobIfComplete(cluster string, application string, jobName string) error {
	cmd := exec.Command("kubectl", "--context", cluster, "--namespace", application, "get", "job", jobName, "-o", "jsonpath='{ .status.succeeded }'")
	out, ce := cmd.Output()
	if ce != nil {
		return ce
	}

	// The command above return the number of succeeded if job is complete otherwise it return the string ''
	if string(out) != "''" {
		fmt.Fprintf(os.Stderr, "The job %s is complete. We delete it and create new one\n", jobName)
		return deleteJob(cluster, application, jobName)
	}
	return nil
}

func attachContainer(cluster string, application string, jobCtx *JobContext) error {
	cmd := exec.Command("kubectl", "--context", cluster, "--namespace", application, "attach", "-it", jobCtx.Pod, "-c", jobCtx.Container)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Use Ctrl-P,Ctrl-Q to detach from container")

	return cmd.Run()
}

func waitPod(cluster string, application string, jobContext *JobContext) error {
	tries := 1
	maxTries := 180

	for tries <= maxTries {
		out, ce := getPodState(cluster, application, jobContext)

		if ce != nil {
			if ee, ok := ce.(*exec.ExitError); ok {
				fmt.Fprintf(os.Stderr, "ee kubectl error: %s", ee.Stderr)
			}
			return ce
		}

		if strings.Contains(out, "Running") || strings.Contains(out, "Failed") || strings.Contains(out, "Succeeded") {
			break
		}

		fmt.Fprintf(os.Stderr, "\033[2K\rpod not ready: %s (%d/%d)", out, tries, maxTries)

		time.Sleep(1 * time.Second)
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

// Get map of Apps, compares it with the app filter given by the -a flag for existence, and fetch the configmap if exists
func getConfigMap(application string, cluster string) ([]byte, error) {
	cmdContext, _ := context.WithTimeout(context.Background(), 5*time.Second)

	//defer cancel()
	configmap, err := exec.CommandContext(cmdContext, "kubectl", "--context", cluster, "--namespace", application, "get", "configmap", "-l", "richman/role=job-template,app.kubernetes.io/name="+application, "-o", "jsonpath={ .items[0].data.template }").Output()
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
