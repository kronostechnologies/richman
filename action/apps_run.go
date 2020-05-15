package action

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/Masterminds/sprig"
	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"text/template"
	"time"
)

type AppsRun struct {
	Filename    string
	Application string
	Config      map[string]string
}

type KubeContext struct {
	Context string
	Namespace string
	Application string
}

type JobContext struct {
	Name string
	Pod string
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

func (c *AppsRun) Run() error {
	data, err := toml.LoadFile(c.Filename)
	if err != nil {
		return err
	}

	kubeContext := &KubeContext{
		Context: data.Get("settings.kubeContext").(string),
		Namespace: data.Get("apps." + c.Application + ".namespace").(string),
		Application: c.Application,
	}

	configMap, err := getConfigMap(kubeContext)
	if err != nil {
		return err
	}

	jobTemplate := template.Must(template.New("configmap").Funcs(sprig.TxtFuncMap()).Parse(configMap))
	templateFields := ListTemplateFields(jobTemplate)

	user, err := getUser(kubeContext)
	if err != nil {
		return err
	}

	c.Config["name"] = fmt.Sprintf("%s-%s", c.Application, user)

	if ve := validateConfig(templateFields, c.Config); ve != nil {
		return ve
	}

	buf := new(bytes.Buffer)
	if te := jobTemplate.Execute(buf, c.Config); te != nil {
		return te
	}

	jobYaml, ae := applyConfig(kubeContext, buf)
	if ae != nil {
		return ae
	}

	jobContext, pe := getJobContext(kubeContext, jobYaml)
	if pe != nil {
		return pe
	}

	channel := make(chan os.Signal)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-channel
		deleteJob(kubeContext, jobContext)
		os.Exit(1)
	}()

	wpe := waitPod(kubeContext, jobContext)

	deleteOnExit := false

	if wpe == nil {
		attach := true
		for attach {
			ace := attachContainer(kubeContext, jobContext)

			time.Sleep(1 * time.Second)

			state, _ := getPodState(kubeContext, jobContext)
			if strings.Contains(state, "Succeeded") {
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
		return deleteJob(kubeContext, jobContext)
	}

	return nil
}

func promptChoice(message string) string {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Fprintf(os.Stderr, "\n%s ", strings.TrimSpace(message))
	scanner.Scan()

	return strings.TrimSpace(scanner.Text())
}

func getPodState(kubeContext *KubeContext, jobContext *JobContext) (string, error) {
	cmd := exec.Command("kubectl","--context", kubeContext.Context, "--namespace", kubeContext.Namespace, "get", "pod", jobContext.Pod, "-o", "jsonpath='{ .status.phase }'")
	out, ce := cmd.Output()
	return string(out), ce
}

func waitPod(kubeContext *KubeContext, jobContext *JobContext) error {
	tries := 1
	maxTries := 75

	for tries <= maxTries {
		out, ce := getPodState(kubeContext, jobContext)

		if ce != nil {
			if ee, ok := ce.(*exec.ExitError); ok {
				fmt.Fprintf(os.Stderr, "kubectl error: %s", ee.Stderr)
			}
			return ce
		}

		if strings.Contains(out, "Running") {
			break
		}

		fmt.Fprintf(os.Stderr, "pod not ready: %s (%d/%d)\n", out, tries, maxTries)

		time.Sleep(3 * time.Second)
		tries += 1
	}

	return nil
}

func attachContainer(kubeCtx *KubeContext, jobCtx *JobContext) error {
	cmd := exec.Command("kubectl","--context", kubeCtx.Context, "--namespace", kubeCtx.Namespace, "attach", "-it", jobCtx.Pod, "-c", jobCtx.Container)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Use Ctrl-P,Ctrl-Q to detach from container")

	return cmd.Run()
}


func deleteJob(kubeCtx *KubeContext, jobCtx *JobContext) error {
	cmd := exec.Command("kubectl","--context", kubeCtx.Context, "--namespace", kubeCtx.Namespace, "delete", "job", jobCtx.Name)
	out, ce := cmd.Output()
	fmt.Println(string(out))

	if ce != nil {
		if ee, ok := ce.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "kubectl error: %s", ee.Stderr)
		}
		return ce
	}

	return nil
}

func getJobContext(ctx *KubeContext, jobYaml []byte) (*JobContext, error) {
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

    cmd := exec.Command("kubectl","--context", ctx.Context, "--namespace", ctx.Namespace, "get", "pod", "--selector=job-name=" + jobName, "-o", "jsonpath={ .items[0].metadata.name }")
	out, ce := cmd.Output()
	podName := strings.TrimSpace(string(out))

	if ce != nil {
		if ee, ok := ce.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "kubectl error: %s", ee.Stderr)
		}
		return nil, ce
	}

	return &JobContext{
		Name: jobName,
		Container: containerName,
		Pod: podName,
	}, nil
}

func applyConfig(ctx *KubeContext, yamlContent *bytes.Buffer) ([]byte, error) {
	cmd := exec.Command("kubectl","--context", ctx.Context, "--namespace", ctx.Namespace, "apply", "-f", "-", "-o", "yaml")
	cmd.Stdin = yamlContent
	out, err := cmd.Output()

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "kubectl error: %s", ee.Stderr)
		}
	}

	return out, err
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

func getConfigMap(ctx *KubeContext) (string, error) {
	cmdContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cmdContext, "kubectl", "--context", ctx.Context, "--namespace", ctx.Namespace, "get", "configmap", "-l", "richman/role=job-template,app.kubernetes.io/name="+ctx.Application, "-o", "jsonpath={ .items[0].data.template }").Output()

	if err != nil {
		if ce := cmdContext.Err(); ce != nil {
			fmt.Fprintf(os.Stderr, "%s\n", ce)
		}
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "kubectl error: %s", ee.Stderr)
		}
		return "", err
	}

	return string(out), nil
}

func getUser(ctx *KubeContext) (string, error) {
	findKubeUserRegex := regexp.MustCompile(`:([^:@]+)@`)

	out, err := exec.Command("kubectl", "config", "view", "-o", "jsonpath={ .contexts[?(@.name == \""+ctx.Context+"\")].context.user }").Output()
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
