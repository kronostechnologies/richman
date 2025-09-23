package action

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
)

type AppsExec struct {
	Application string
	Command     string
	Config      map[string]string
}

func (c *AppsExec) Run() error {
	//Create a new connection
	clientSet := &Connection{
		KubeConfigPath: GetKubeConfigPath(),
		Cluster:        GetClusterName(),
		ClientSet:      GetClientSet(GetKubeConfigPath()),
	}
	currentContext := clientSet.Cluster
	listApps, err := ListApps(clientSet.ClientSet, c.Application)

	if err != nil {
		return err
	}

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

	// Add the command to the config - split the command into separate arguments
	commandParts := strings.Fields(c.Command)
	// Format as JSON array string for the template
	commandArray := `["` + strings.Join(commandParts, `", "`) + `"]`
	c.Config["command"] = commandArray

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

	jobName, je := getJobName(tplConfig)
	if je != nil {
		return je
	}
	deleteJobIfComplete(currentApp, jobName)

	jobContext, pe := getJobContext(currentApp, tplConfig)
	jobConfigMap, ae := applyConfig(currentApp, tplConfig)
	if ae != nil {
		// Ask to user if he wants to delete the job
		c := promptChoice("kubectl has failed to replace the existing job. Do you want to delete it (y/[n])?")
		if len(c) == 0 {
			c = "n"
		}
		if []rune(strings.ToLower(c))[0] == 'y' {
			de := deleteJob(currentApp, jobName)
			if de != nil {
				if ee, ok := de.(*exec.ExitError); ok {
					fmt.Fprintf(os.Stderr, "deleteJob kubectl error: %s", ee.Stderr)
				}
				return de
			}
		} else {
			return ae
		}

		jobConfigMap, ae = applyConfig(currentApp, tplConfig)
		if ae != nil {
			return ae
		}
	}
	jobContext, pe = getJobContext(currentApp, jobConfigMap)
	if pe != nil {
		if ee, ok := pe.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "GetJobContext kubectl error: %s", ee.Stderr)
		}
	}

	// Setup signal handling for cleanup
	channel := make(chan os.Signal)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-channel
		deleteJob(currentApp, jobName)
		os.Exit(1)
	}()

	// Wait for pod to be ready (including succeeded state for quick jobs)
	wpe := waitPod(currentApp, jobContext)
	if wpe != nil {
		return wpe
	}

	// Check pod status and print it
	status, err := getPodState(currentApp, jobContext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting pod status: %v\n", err)
	} else {
		fmt.Printf("Pod status: %s\n", strings.Trim(status, "'"))
	}

	if strings.Contains(status, "Failed") {
		getPodFailureDetails(currentApp, jobContext)
	}

	// Get logs (including completed logs) and follow if still running
	err = getLogs(currentApp, jobContext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting logs: %v\n", err)
	}

	// Wait for job completion and then delete
	err = waitForJobCompletion(currentApp, jobName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error waiting for job completion: %v\n", err)
	}

	return deleteJob(currentApp, jobName)
}

// getLogs gets the logs from the pod, following if still running
func getLogs(currentApp App, jobCtx *JobContext) error {
	time.Sleep(1 * time.Second)

	fmt.Printf("Getting logs for pod %s, container %s\n", jobCtx.Pod, jobCtx.Container)
	
	var cmd *exec.Cmd
	// Check pod status to determine if we should follow logs or just get them
	status, err := getPodState(currentApp, jobCtx)
	if err != nil {
		return err
	}
	if strings.Contains(status, "Running") {
		// Container is still running, follow logs
		cmd = exec.Command("kubectl", "--context", currentApp.KubeContext.Cluster, "--namespace", currentApp.KubeContext.Namespace, "logs", "-f", jobCtx.Pod, "-c", jobCtx.Container)
		fmt.Println("Use Ctrl+C to stop following logs and clean up")
	} else {
		cmd = exec.Command("kubectl", "--context", currentApp.KubeContext.Cluster, "--namespace", currentApp.KubeContext.Namespace, "logs", jobCtx.Pod, "-c", jobCtx.Container)
	}
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

// waitForJobCompletion waits for the job to complete (either succeed or fail)
func waitForJobCompletion(currentApp App, jobName string) error {
	for {
		// Check if job completed successfully
		cmd := exec.Command("kubectl", "--context", currentApp.KubeContext.Cluster, "--namespace", currentApp.KubeContext.Namespace, "get", "job", jobName, "-o", "jsonpath='{.status.conditions[?(@.type==\"Complete\")].status}'")
		out, err := cmd.Output()
		if err != nil {
			return err
		}

		completeStatus := strings.Trim(string(out), "'")
		if strings.Contains(completeStatus, "True") {
			fmt.Println("Job completed successfully")
			return nil
		}

		// Check if job failed
		cmd = exec.Command("kubectl", "--context", currentApp.KubeContext.Cluster, "--namespace", currentApp.KubeContext.Namespace, "get", "job", jobName, "-o", "jsonpath='{.status.conditions[?(@.type==\"Failed\")].status}'")
		out, err = cmd.Output()
		if err != nil {
			return err
		}

		failedStatus := strings.Trim(string(out), "'")
		if strings.Contains(failedStatus, "True") {
			fmt.Println("Job failed")
			return fmt.Errorf("job failed")
		}

		time.Sleep(2 * time.Second)
	}
}

// getPodFailureDetails gets and prints the failure reason and message for a failed pod
func getPodFailureDetails(currentApp App, jobContext *JobContext) {
	// Get pod failure reason
	reasonCmd := exec.Command("kubectl", "--context", currentApp.KubeContext.Cluster, "--namespace", currentApp.KubeContext.Namespace, "get", "pod", jobContext.Pod, "-o", "jsonpath='{.status.containerStatuses[0].state.terminated.reason}'")
	reasonOut, reasonErr := reasonCmd.Output()
	
	messageCmd := exec.Command("kubectl", "--context", currentApp.KubeContext.Cluster, "--namespace", currentApp.KubeContext.Namespace, "get", "pod", jobContext.Pod, "-o", "jsonpath='{.status.containerStatuses[0].state.terminated.message}'")
	messageOut, messageErr := messageCmd.Output()
	
	if reasonErr == nil && len(reasonOut) > 0 {
		reason := strings.Trim(string(reasonOut), "'")
		if reason != "" {
			fmt.Printf("Pod failure reason: %s\n", reason)
		}
	}
	if messageErr == nil && len(messageOut) > 0 {
		message := strings.Trim(string(messageOut), "'")
		if message != "" {
			fmt.Printf("Pod failure message: %s\n", message)
		}
	}
}