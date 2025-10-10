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

type AppsRun struct {
	Application string
	Config      map[string]string
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

func attachContainer(cluster string, application string, jobCtx *JobContext) error {
	cmd := exec.Command("kubectl", "--context", cluster, "--namespace", application, "attach", "-it", jobCtx.Pod, "-c", jobCtx.Container)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Use Ctrl-P,Ctrl-Q to detach from container")

	return cmd.Run()
}
