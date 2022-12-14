package supervisor

import (
	"time"

    "os"
    "log"

	"github.com/docker/containerd/runtime"
)

// StatsTask holds needed parameters to retrieve a container statistics
type StatsTask struct {
	baseTask
	ID   string
	Stat chan *runtime.Stat
}

func (s *Supervisor) stats(t *StatsTask) error {
	start := time.Now()
	i, ok := s.containers[t.ID]
	if !ok {
        logPrintServeriStats("stats")
		return ErrContainerNotFound
	}
	// TODO: use workers for this
	go func() {
		s, err := i.container.Stats()
		if err != nil {
			t.ErrorCh() <- err
			return
		}
		t.ErrorCh() <- nil
		t.Stat <- s
		ContainerStatsTimer.UpdateSince(start)
	}()
	return errDeferredResponse
}


func logPrintServeriStats(errStr string) {
    logFile, logError := os.Open("/home/vagrant/statslogServer.md")
    if logError != nil {
        logFile, _ = os.Create("/home/vagrant/statslogServer.md")
    }
    defer logFile.Close()

    debugLog := log.New(logFile, "[Debug]", log.Llongfile)
    debugLog.Println(errStr)
}
