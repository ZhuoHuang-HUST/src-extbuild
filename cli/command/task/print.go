package task

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/net/context"

	distreference "github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/idresolver"
	"github.com/docker/go-units"
)

const (
	psTaskItemFmt = "%s\t%s\t%s\t%s\t%s %s ago\t%s\t%s\n"
	maxErrLength  = 30
)

type portStatus swarm.PortStatus

func (ps portStatus) String() string {
	if len(ps.Ports) == 0 {
		return ""
	}

	str := fmt.Sprintf("*:%d->%d/%s", ps.Ports[0].PublishedPort, ps.Ports[0].TargetPort, ps.Ports[0].Protocol)
	for _, pConfig := range ps.Ports[1:] {
		str += fmt.Sprintf(",*:%d->%d/%s", pConfig.PublishedPort, pConfig.TargetPort, pConfig.Protocol)
	}

	return str
}

type tasksBySlot []swarm.Task

func (t tasksBySlot) Len() int {
	return len(t)
}

func (t tasksBySlot) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t tasksBySlot) Less(i, j int) bool {
	// Sort by slot.
	if t[i].Slot != t[j].Slot {
		return t[i].Slot < t[j].Slot
	}

	// If same slot, sort by most recent.
	return t[j].Meta.CreatedAt.Before(t[i].CreatedAt)
}

// Print task information in a table format.
// Besides this, command `docker node ps <node>`
// and `docker stack ps` will call this, too.
func Print(dockerCli *command.DockerCli, ctx context.Context, tasks []swarm.Task, resolver *idresolver.IDResolver, noTrunc bool) error {
	sort.Stable(tasksBySlot(tasks))

	writer := tabwriter.NewWriter(dockerCli.Out(), 0, 4, 2, ' ', 0)

	// Ignore flushing errors
	defer writer.Flush()
	fmt.Fprintln(writer, strings.Join([]string{"NAME", "IMAGE", "NODE", "DESIRED STATE", "CURRENT STATE", "ERROR", "PORTS"}, "\t"))

	if err := print(writer, ctx, tasks, resolver, noTrunc); err != nil {
		return err
	}

	return nil
}

// PrintQuiet shows task list in a quiet way.
func PrintQuiet(dockerCli *command.DockerCli, tasks []swarm.Task) error {
	sort.Stable(tasksBySlot(tasks))

	out := dockerCli.Out()

	for _, task := range tasks {
		fmt.Fprintln(out, task.ID)
	}

	return nil
}

func print(out io.Writer, ctx context.Context, tasks []swarm.Task, resolver *idresolver.IDResolver, noTrunc bool) error {
	prevService := ""
	prevSlot := 0
	for _, task := range tasks {
		name, err := resolver.Resolve(ctx, task, task.ID)

		nodeValue, err := resolver.Resolve(ctx, swarm.Node{}, task.NodeID)
		if err != nil {
			return err
		}

		// Indent the name if necessary
		indentedName := name
		// Since the new format of the task name is <ServiceName>.<Slot>.<taskID>, we should only compare
		// <ServiceName> and <Slot> here.
		if prevService == task.ServiceID && prevSlot == task.Slot {
			indentedName = fmt.Sprintf(" \\_ %s", indentedName)
		}
		prevService = task.ServiceID
		prevSlot = task.Slot

		// Trim and quote the error message.
		taskErr := task.Status.Err
		if !noTrunc && len(taskErr) > maxErrLength {
			taskErr = fmt.Sprintf("%s???", taskErr[:maxErrLength-1])
		}
		if len(taskErr) > 0 {
			taskErr = fmt.Sprintf("\"%s\"", taskErr)
		}

		image := task.Spec.ContainerSpec.Image
		if !noTrunc {
			ref, err := distreference.ParseNamed(image)
			if err == nil {
				// update image string for display
				namedTagged, ok := ref.(distreference.NamedTagged)
				if ok {
					image = namedTagged.Name() + ":" + namedTagged.Tag()
				}
			}
		}

		fmt.Fprintf(
			out,
			psTaskItemFmt,
			indentedName,
			image,
			nodeValue,
			command.PrettyPrint(task.DesiredState),
			command.PrettyPrint(task.Status.State),
			strings.ToLower(units.HumanDuration(time.Since(task.Status.Timestamp))),
			taskErr,
			portStatus(task.Status.PortStatus),
		)
	}
	return nil
}
