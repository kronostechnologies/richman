package action

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-yaml/yaml"
)

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
