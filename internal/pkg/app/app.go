package app

import "github.com/stakater/ProxyInjector/internal/pkg/cmd"

// Run runs the command
func Run() error {
	command := cmd.NewProxyInjectorCommand()
	return command.Execute()
}
