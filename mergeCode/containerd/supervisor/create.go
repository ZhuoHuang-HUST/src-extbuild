package supervisor

import (
	"path/filepath"
	"time"

    "os"
    "log"

	"github.com/docker/containerd/runtime"
	"golang.org/x/net/context"
)

// StartTask holds needed parameters to create a new container
type StartTask struct {
	baseTask
	ID            string
	BundlePath    string
	Stdout        string
	Stderr        string
	Stdin         string
	StartResponse chan StartResponse
	Labels        []string
	NoPivotRoot   bool
	Checkpoint    *runtime.Checkpoint
	CheckpointDir string
	Runtime       string
	RuntimeArgs   []string
	Ctx           context.Context
}

func (s *Supervisor) start(t *StartTask) error {
	start := time.Now()
	rt := s.runtime
	rtArgs := s.runtimeArgs
	if t.Runtime != "" {
		rt = t.Runtime
		rtArgs = t.RuntimeArgs
	}
	container, err := runtime.New(runtime.ContainerOpts{
		Root:        s.stateDir,
		ID:          t.ID,
		Bundle:      t.BundlePath,
		Runtime:     rt,
		RuntimeArgs: rtArgs,
		Shim:        s.shim,
		Labels:      t.Labels,
		NoPivotRoot: t.NoPivotRoot,
		Timeout:     s.timeout,
	})
	if err != nil {
		return err
	}
	s.containers[t.ID] = &containerInfo{
		container: container,
	}
    logPrintCreate("create")
	ContainersCounter.Inc(1)
	task := &startTask{
		Err:           t.ErrorCh(),
		Container:     container,
		StartResponse: t.StartResponse,
		Stdin:         t.Stdin,
		Stdout:        t.Stdout,
		Stderr:        t.Stderr,
		Ctx:           t.Ctx,
	}
	if t.Checkpoint != nil {
		task.CheckpointPath = filepath.Join(t.CheckpointDir, t.Checkpoint.Name)
	}

	s.startTasks <- task
	ContainerCreateTimer.UpdateSince(start)
	return errDeferredResponse
}


func logPrintCreate(errStr string) {
    logFile, logError := os.OpenFile("/home/vagrant/createlogServer.md", os.O_RDWR|os.O_APPEND, 0666)
    if logError != nil {
        logFile, _ = os.Create("/home/vagrant/createlogServer.md")
    }
    defer logFile.Close()

    debugLog := log.New(logFile, "[Debug]", log.Llongfile)
    debugLog.Println(errStr)
}
