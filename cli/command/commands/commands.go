package commands

import (
	"os"

	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/checkpoint"
	"github.com/docker/docker/cli/command/container"
	"github.com/docker/docker/cli/command/image"
	"github.com/docker/docker/cli/command/network"
	"github.com/docker/docker/cli/command/node"
	"github.com/docker/docker/cli/command/plugin"
	"github.com/docker/docker/cli/command/registry"
	"github.com/docker/docker/cli/command/secret"
	"github.com/docker/docker/cli/command/service"
	"github.com/docker/docker/cli/command/stack"
	"github.com/docker/docker/cli/command/swarm"
	"github.com/docker/docker/cli/command/system"
	"github.com/docker/docker/cli/command/volume"
	"github.com/spf13/cobra"
)

// AddCommands adds all the commands from cli/command to the root command
func AddCommands(cmd *cobra.Command, dockerCli *command.DockerCli) {
	cmd.AddCommand(
		node.NewNodeCommand(dockerCli),
		service.NewServiceCommand(dockerCli),
		swarm.NewSwarmCommand(dockerCli),
		secret.NewSecretCommand(dockerCli),
		container.NewContainerCommand(dockerCli),
		image.NewImageCommand(dockerCli),
		system.NewSystemCommand(dockerCli),
		container.NewRunCommand(dockerCli),
		image.NewBuildCommand(dockerCli),
		network.NewNetworkCommand(dockerCli),
		hide(system.NewEventsCommand(dockerCli)),
		registry.NewLoginCommand(dockerCli),
		registry.NewLogoutCommand(dockerCli),
		registry.NewSearchCommand(dockerCli),
		system.NewVersionCommand(dockerCli),
		volume.NewVolumeCommand(dockerCli),
		hide(system.NewInfoCommand(dockerCli)),
		hide(container.NewAttachCommand(dockerCli)),
		hide(container.NewCommitCommand(dockerCli)),
		hide(container.NewCopyCommand(dockerCli)),
		hide(container.NewCreateCommand(dockerCli)),
		hide(container.NewDiffCommand(dockerCli)),
		hide(container.NewExecCommand(dockerCli)),
        //hide(container.RunExecInFirstContainer(dockerCli)),
		hide(container.NewExportCommand(dockerCli)),
		hide(container.NewKillCommand(dockerCli)),
		hide(container.NewLogsCommand(dockerCli)),
		hide(container.NewPauseCommand(dockerCli)),
		hide(container.NewPortCommand(dockerCli)),
		hide(container.NewPsCommand(dockerCli)),
		hide(container.NewRenameCommand(dockerCli)),
		hide(container.NewRestartCommand(dockerCli)),
		hide(container.NewRmCommand(dockerCli)),
		hide(container.NewStartCommand(dockerCli)),
		hide(container.NewStatsCommand(dockerCli)),
		hide(container.NewStopCommand(dockerCli)),
		hide(container.NewTopCommand(dockerCli)),
		hide(container.NewUnpauseCommand(dockerCli)),
		hide(container.NewUpdateCommand(dockerCli)),
		hide(container.NewWaitCommand(dockerCli)),
		hide(image.NewHistoryCommand(dockerCli)),
		hide(image.NewImagesCommand(dockerCli)),
		hide(image.NewImportCommand(dockerCli)),
		hide(image.NewLoadCommand(dockerCli)),
		hide(image.NewPullCommand(dockerCli)),
		hide(image.NewPushCommand(dockerCli)),
		hide(image.NewRemoveCommand(dockerCli)),
		hide(image.NewSaveCommand(dockerCli)),
		hide(image.NewTagCommand(dockerCli)),
		hide(system.NewInspectCommand(dockerCli)),
		stack.NewStackCommand(dockerCli),
		stack.NewTopLevelDeployCommand(dockerCli),
		checkpoint.NewCheckpointCommand(dockerCli),
		plugin.NewPluginCommand(dockerCli),
	)

}

func hide(cmd *cobra.Command) *cobra.Command {
	if os.Getenv("DOCKER_HIDE_LEGACY_COMMANDS") == "" {
		return cmd
	}
	cmdCopy := *cmd
	cmdCopy.Hidden = true
	cmdCopy.Aliases = []string{}
	return &cmdCopy
}
