// +build linux solaris

package libcontainerd

import (
	"encoding/json"
    "fmt"
    "io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	containerd "github.com/docker/containerd/api/grpc/types"
	"github.com/docker/docker/pkg/ioutils"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/tonistiigi/fifo"
	"golang.org/x/net/context"
)

type container struct {
	containerCommon

	// Platform specific fields are below here.
	pauseMonitor
	oom         bool
	runtime     string
	runtimeArgs []string
//    isBuilding  bool
}

type runtime struct {
	path string
	args []string
}

// WithRuntime sets the runtime to be used for the created container
func WithRuntime(path string, args []string) CreateOption {
	return runtime{path, args}
}

func (rt runtime) Apply(p interface{}) error {
	if pr, ok := p.(*container); ok {
		pr.runtime = rt.path
		pr.runtimeArgs = rt.args
	}
	return nil
}

func (ctr *container) clean() error {
	if os.Getenv("LIBCONTAINERD_NOCLEAN") == "1" {
		return nil
	}
	if _, err := os.Lstat(ctr.dir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := os.RemoveAll(ctr.dir); err != nil {
		return err
	}
	return nil
}

// cleanProcess removes the fifos used by an additional process.
// Caller needs to lock container ID before calling this method.
func (ctr *container) cleanProcess(id string) {
	if p, ok := ctr.processes[id]; ok {
		for _, i := range []int{syscall.Stdin, syscall.Stdout, syscall.Stderr} {
			if err := os.Remove(p.fifo(i)); err != nil && !os.IsNotExist(err) {
				logrus.Warnf("libcontainerd: failed to remove %v for process %v: %v", p.fifo(i), id, err)
			}
		}
	}
	delete(ctr.processes, id)
}

func (ctr *container) spec() (*specs.Spec, error) {
	var spec specs.Spec
	dt, err := ioutil.ReadFile(filepath.Join(ctr.dir, configFilename))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(dt, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func (ctr *container) start(checkpoint string, checkpointDir string, attachStdio StdioCallback) error {
	spec, err := ctr.spec()
	if err != nil {
		return nil
	}

    fmt.Println("libcontainerd/container_unix.go     start")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})

	iopipe, err := ctr.openFifos(spec.Process.Terminal)
	if err != nil {
		return err
	}
    
    fmt.Println("libcontainerd/container_unix.go     openFifos")


	var stdinOnce sync.Once

	// we need to delay stdin closure after container start or else "stdin close"
	// event will be rejected by containerd.
	// stdin closure happens in attachStdio
	stdin := iopipe.Stdin
	iopipe.Stdin = ioutils.NewWriteCloserWrapper(stdin, func() error {
		var err error
		stdinOnce.Do(func() { // on error from attach we don't know if stdin was already closed
			err = stdin.Close()
			go func() {
				select {
				case <-ready:
				case <-ctx.Done():
				}
				select {
				case <-ready:
					if err := ctr.sendCloseStdin(); err != nil {
						logrus.Warnf("failed to close stdin: %+v", err)
					}
				default:
				}
			}()
		})
		return err
	})

    fmt.Println("libcontainerd/container_unix.go     close stdin")

	r := &containerd.CreateContainerRequest{
		Id:            ctr.containerID,
		BundlePath:    ctr.dir,
		Stdin:         ctr.fifo(syscall.Stdin),
		Stdout:        ctr.fifo(syscall.Stdout),
		Stderr:        ctr.fifo(syscall.Stderr),
		Checkpoint:    checkpoint,
		CheckpointDir: checkpointDir,
		// check to see if we are running in ramdisk to disable pivot root
		NoPivotRoot: os.Getenv("DOCKER_RAMDISK") != "",
		Runtime:     ctr.runtime,
		RuntimeArgs: ctr.runtimeArgs,
	}
	ctr.client.appendContainer(ctr)

    fmt.Println("libcontainered/container_unix.go Create()  attachStdio StdioCallback")
	if err := attachStdio(*iopipe); err != nil {
		ctr.closeFifos(iopipe)
		return err
	}

	resp, err := ctr.client.remote.apiClient.CreateContainer(context.Background(), r)
    fmt.Println("libcontainerd/container_unix.go     apiClient CreateContainer")
    if err != nil {
		ctr.closeFifos(iopipe)
		return err
	}
	ctr.systemPid = systemPid(resp.Container)
	close(ready)

    fmt.Println("libcontainerd/container_unix.go     start to sleep 10 seconds")
    time.Sleep(time.Second * 10)
    fmt.Println("libcontainerd/container_unix.go     sleep end")


	return ctr.client.backend.StateChanged(ctr.containerID, StateInfo{
		CommonStateInfo: CommonStateInfo{
			State: StateStart,
			Pid:   ctr.systemPid,
		}})
}

func (ctr *container) newProcess(friendlyName string) *process {
	return &process{
		dir: ctr.dir,
		processCommon: processCommon{
			containerID:  ctr.containerID,
			friendlyName: friendlyName,
			client:       ctr.client,
		},
	}
}

func (ctr *container) handleEvent(e *containerd.Event) error {
    fmt.Println("libcontainerd/container_unix.go  handleEvent()")
	ctr.client.lock(ctr.containerID)
	defer ctr.client.unlock(ctr.containerID)
	switch e.Type {
	case StateExit, StatePause, StateResume, StateOOM:
		st := StateInfo{
			CommonStateInfo: CommonStateInfo{
				State:    e.Type,
				ExitCode: e.Status,
			},
			OOMKilled: e.Type == StateExit && ctr.oom,
		}
		if e.Type == StateOOM {
			ctr.oom = true
		}
		if e.Type == StateExit && e.Pid != InitFriendlyName {
			st.ProcessID = e.Pid
			st.State = StateExitProcess
		}

		// Remove process from list if we have exited
		switch st.State {
		case StateExit:
			ctr.clean()
			ctr.client.deleteContainer(e.Id)
            fmt.Println("libcontainerd/container_unix.go/handleEvent()  deleteContainer ", e.Id)
		case StateExitProcess:
			ctr.cleanProcess(st.ProcessID)
		}
		ctr.client.q.append(e.Id, func() {
            fmt.Println("libcontainerd/container_unix.go/handleEvent()  StateChanged")
			eErr := ctr.client.backend.StateChanged(e.Id, st)
            if eErr != nil {
				logrus.Errorf("libcontainerd: backend.StateChanged(): %v", eErr)
                fmt.Println("libcontainered/container_unix.go/handleEvent() err")
			}
            fmt.Println("libcontainerd/container_unix.go/handleEvent()  Status : ", e.Type)
			if e.Type == StatePause || e.Type == StateResume {
				ctr.pauseMonitor.handle(e.Type)
			}
			if e.Type == StateExit {
				if en := ctr.client.getExitNotifier(e.Id); en != nil {
					en.close()
				}
			}
		})

	default:
		logrus.Debugf("libcontainerd: event unhandled: %+v", e)
	}
	return nil
}

// discardFifos attempts to fully read the container fifos to unblock processes
// that may be blocked on the writer side.
func (ctr *container) discardFifos() {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	for _, i := range []int{syscall.Stdout, syscall.Stderr} {
		f, err := fifo.OpenFifo(ctx, ctr.fifo(i), syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			logrus.Warnf("error opening fifo %v for discarding: %+v", f, err)
			continue
		}
		go func() {
			io.Copy(ioutil.Discard, f)
		}()
	}
}
