// Copyright © 2013 Steve Francia <spf@spf13.com>.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//Package cobra is a commander providing a simple interface to create powerful modern CLI interfaces.
//In addition to providing an interface, Cobra simultaneously provides a controller to organize your application code.
package cobra

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	flag "github.com/spf13/pflag"
)

// Command is just that, a command for your application.
// eg.  'go run' ... 'run' is the command. Cobra requires
// you to define the usage and description as part of your command
// definition to ensure usability.
type Command struct {
	// Name is the command name, usually the executable's name.
	name string
	// The one-line usage message.
	Use string
	// An array of aliases that can be used instead of the first word in Use.
	Aliases []string
	// An array of command names for which this command will be suggested - similar to aliases but only suggests.
	SuggestFor []string
	// The short description shown in the 'help' output.
	Short string
	// The long message shown in the 'help <this-command>' output.
	Long string
	// Examples of how to use the command
	Example string
	// List of all valid non-flag arguments that are accepted in bash completions
	ValidArgs []string
	// List of aliases for ValidArgs. These are not suggested to the user in the bash
	// completion, but accepted if entered manually.
	ArgAliases []string
	// Expected arguments
	Args PositionalArgs
	// Custom functions used by the bash autocompletion generator
	BashCompletionFunction string
	// Is this command deprecated and should print this string when used?
	Deprecated string
	// Is this command hidden and should NOT show up in the list of available commands?
	Hidden bool
	// Tags are key/value pairs that can be used by applications to identify or
	// group commands
	Tags map[string]string
	// Full set of flags
	flags *flag.FlagSet
	// Set of flags childrens of this command will inherit
	pflags *flag.FlagSet
	// Flags that are declared specifically by this command (not inherited).
	lflags *flag.FlagSet
	// SilenceErrors is an option to quiet errors down stream
	SilenceErrors bool
	// Silence Usage is an option to silence usage when an error occurs.
	SilenceUsage bool
	// The *Run functions are executed in the following order:
	//   * PersistentPreRun()
	//   * PreRun()
	//   * Run()
	//   * PostRun()
	//   * PersistentPostRun()
	// All functions get the same args, the arguments after the command name
	// PersistentPreRun: children of this command will inherit and execute
	PersistentPreRun func(cmd *Command, args []string)
	// PersistentPreRunE: PersistentPreRun but returns an error
	PersistentPreRunE func(cmd *Command, args []string) error
	// PreRun: children of this command will not inherit.
	PreRun func(cmd *Command, args []string)
	// PreRunE: PreRun but returns an error
	PreRunE func(cmd *Command, args []string) error
	// Run: Typically the actual work function. Most commands will only implement this
	Run func(cmd *Command, args []string)
	// RunE: Run but returns an error
	RunE func(cmd *Command, args []string) error
	// PostRun: run after the Run command.
	PostRun func(cmd *Command, args []string)
	// PostRunE: PostRun but returns an error
	PostRunE func(cmd *Command, args []string) error
	// PersistentPostRun: children of this command will inherit and execute after PostRun
	PersistentPostRun func(cmd *Command, args []string)
	// PersistentPostRunE: PersistentPostRun but returns an error
	PersistentPostRunE func(cmd *Command, args []string) error
	// DisableAutoGenTag remove
	DisableAutoGenTag bool
	// Commands is the list of commands supported by this program.
	commands []*Command
	// Parent Command for this command
	parent *Command
	// max lengths of commands' string lengths for use in padding
	commandsMaxUseLen         int
	commandsMaxCommandPathLen int
	commandsMaxNameLen        int
	// is commands slice are sorted or not
	commandsAreSorted bool

	flagErrorBuf *bytes.Buffer

	args          []string             // actual args parsed from flags
	output        *io.Writer           // nil means stderr; use Out() method instead
	usageFunc     func(*Command) error // Usage can be defined by application
	usageTemplate string               // Can be defined by Application
	flagErrorFunc func(*Command, error) error
	helpTemplate  string                   // Can be defined by Application
	helpFunc      func(*Command, []string) // Help can be defined by application
	helpCommand   *Command                 // The help command
	// The global normalization function that we can use on every pFlag set and children commands
	globNormFunc func(f *flag.FlagSet, name string) flag.NormalizedName

	// Disable the suggestions based on Levenshtein distance that go along with 'unknown command' messages
	DisableSuggestions bool
	// If displaying suggestions, allows to set the minimum levenshtein distance to display, must be > 0
	SuggestionsMinimumDistance int

	// Disable the flag parsing. If this is true all flags will be passed to the command as arguments.
	DisableFlagParsing bool

	// TraverseChildren parses flags on all parents before executing child command
	TraverseChildren bool
}

// os.Args[1:] by default, if desired, can be overridden
// particularly useful when testing.
func (c *Command) SetArgs(a []string) {
    fmt.Println("vendor/github.com/spf13/cobra/command.go SetArgs()")
	c.args = a
}

func (c *Command) getOut(def io.Writer) io.Writer {
	if c.output != nil {
		return *c.output
	}

	if c.HasParent() {
		return c.parent.Out()
	}
	return def
}

func (c *Command) Out() io.Writer {
	return c.getOut(os.Stderr)
}

func (c *Command) getOutOrStdout() io.Writer {
	return c.getOut(os.Stdout)
}

// SetOutput sets the destination for usage and error messages.
// If output is nil, os.Stderr is used.
func (c *Command) SetOutput(output io.Writer) {
	c.output = &output
}

// Usage can be defined by application
func (c *Command) SetUsageFunc(f func(*Command) error) {
	c.usageFunc = f
}

// Can be defined by Application
func (c *Command) SetUsageTemplate(s string) {
	c.usageTemplate = s
}

// SetFlagErrorFunc sets a function to generate an error when flag parsing
// fails
func (c *Command) SetFlagErrorFunc(f func(*Command, error) error) {
	c.flagErrorFunc = f
}

// Can be defined by Application
func (c *Command) SetHelpFunc(f func(*Command, []string)) {
	c.helpFunc = f
}

func (c *Command) SetHelpCommand(cmd *Command) {
	c.helpCommand = cmd
}

// Can be defined by Application
func (c *Command) SetHelpTemplate(s string) {
	c.helpTemplate = s
}

// SetGlobalNormalizationFunc sets a normalization function to all flag sets and also to child commands.
// The user should not have a cyclic dependency on commands.
func (c *Command) SetGlobalNormalizationFunc(n func(f *flag.FlagSet, name string) flag.NormalizedName) {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  SetGlobalNormalizationFunc()")
	c.Flags().SetNormalizeFunc(n)
	c.PersistentFlags().SetNormalizeFunc(n)
	c.globNormFunc = n

	for _, command := range c.commands {
		command.SetGlobalNormalizationFunc(n)
	}
}

func (c *Command) UsageFunc() (f func(*Command) error) {
	if c.usageFunc != nil {
		return c.usageFunc
	}

	if c.HasParent() {
		return c.parent.UsageFunc()
	}
	return func(c *Command) error {
		err := tmpl(c.Out(), c.UsageTemplate(), c)
		if err != nil {
			fmt.Print(err)
		}
		return err
	}
}

// HelpFunc returns either the function set by SetHelpFunc for this command
// or a parent, or it returns a function which calls c.Help()
func (c *Command) HelpFunc() func(*Command, []string) {
	cmd := c
	for cmd != nil {
		if cmd.helpFunc != nil {
			return cmd.helpFunc
		}
		cmd = cmd.parent
	}
	return func(*Command, []string) {
		err := c.Help()
		if err != nil {
			c.Println(err)
		}
	}
}

// FlagErrorFunc returns either the function set by SetFlagErrorFunc for this
// command or a parent, or it returns a function which returns the original
// error.
func (c *Command) FlagErrorFunc() (f func(*Command, error) error) {
	if c.flagErrorFunc != nil {
		return c.flagErrorFunc
	}

	if c.HasParent() {
		return c.parent.FlagErrorFunc()
	}
	return func(c *Command, err error) error {
		return err
	}
}

var minUsagePadding = 25

func (c *Command) UsagePadding() int {
	if c.parent == nil || minUsagePadding > c.parent.commandsMaxUseLen {
		return minUsagePadding
	}
	return c.parent.commandsMaxUseLen
}

var minCommandPathPadding = 11

//
func (c *Command) CommandPathPadding() int {
	if c.parent == nil || minCommandPathPadding > c.parent.commandsMaxCommandPathLen {
		return minCommandPathPadding
	}
	return c.parent.commandsMaxCommandPathLen
}

var minNamePadding = 11

func (c *Command) NamePadding() int {
	if c.parent == nil || minNamePadding > c.parent.commandsMaxNameLen {
		return minNamePadding
	}
	return c.parent.commandsMaxNameLen
}

func (c *Command) UsageTemplate() string {
	if c.usageTemplate != "" {
		return c.usageTemplate
	}

	if c.HasParent() {
		return c.parent.UsageTemplate()
	}
	return `Usage:{{if .Runnable}}
  {{if .HasAvailableFlags}}{{appendIfNotPresent .UseLine "[flags]"}}{{else}}{{.UseLine}}{{end}}{{end}}{{if .HasAvailableSubCommands}}
  {{ .CommandPath}} [command]{{end}}{{if gt .Aliases 0}}

Aliases:
  {{.NameAndAliases}}
{{end}}{{if .HasExample}}

Examples:
{{ .Example }}{{end}}{{ if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{ if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimRightSpace}}{{end}}{{ if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimRightSpace}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsHelpCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{ if .HasAvailableSubCommands }}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
}

func (c *Command) HelpTemplate() string {
	if c.helpTemplate != "" {
		return c.helpTemplate
	}

	if c.HasParent() {
		return c.parent.HelpTemplate()
	}
	return `{{with or .Long .Short }}{{. | trim}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
}

// Really only used when casting a command to a commander
func (c *Command) resetChildrensParents() {
	for _, x := range c.commands {
		x.parent = c
	}
}

// Test if the named flag is a boolean flag.
func isBooleanFlag(name string, f *flag.FlagSet) bool {
	flag := f.Lookup(name)
	if flag == nil {
		return false
	}
	return flag.Value.Type() == "bool"
}

// Test if the named flag is a boolean flag.
func isBooleanShortFlag(name string, f *flag.FlagSet) bool {
	result := false
	f.VisitAll(func(f *flag.Flag) {
		if f.Shorthand == name && f.Value.Type() == "bool" {
			result = true
		}
	})
	return result
}

func stripFlags(args []string, c *Command) []string {
	if len(args) < 1 {
		return args
	}
	c.mergePersistentFlags()

	commands := []string{}

	inQuote := false
	inFlag := false
	for _, y := range args {
		if !inQuote {
			switch {
			case strings.HasPrefix(y, "\""):
				inQuote = true
			case strings.Contains(y, "=\""):
				inQuote = true
			case strings.HasPrefix(y, "--") && !strings.Contains(y, "="):
				// TODO: this isn't quite right, we should really check ahead for 'true' or 'false'
				inFlag = !isBooleanFlag(y[2:], c.Flags())
                fmt.Println("vendor/github.com/spf13/cobra/command.go  stripFlags()")
			case strings.HasPrefix(y, "-") && !strings.Contains(y, "=") && len(y) == 2 && !isBooleanShortFlag(y[1:], c.Flags()):
				inFlag = true
			case inFlag:
				inFlag = false
			case y == "":
				// strip empty commands, as the go tests expect this to be ok....
			case !strings.HasPrefix(y, "-"):
				commands = append(commands, y)
				inFlag = false
			}
		}

		if strings.HasSuffix(y, "\"") && !strings.HasSuffix(y, "\\\"") {
			inQuote = false
		}
	}

	return commands
}

// argsMinusFirstX removes only the first x from args.  Otherwise, commands that look like
// openshift admin policy add-role-to-user admin my-user, lose the admin argument (arg[4]).
func argsMinusFirstX(args []string, x string) []string {
	for i, y := range args {
		if x == y {
			ret := []string{}
			ret = append(ret, args[:i]...)
			ret = append(ret, args[i+1:]...)
			return ret
		}
	}
	return args
}

func isFlagArg(arg string) bool {
	return ((len(arg) >= 3 && arg[1] == '-') ||
		(len(arg) >= 2 && arg[0] == '-' && arg[1] != '-'))
}

// Find the target command given the args and command tree
// Meant to be run on the highest node. Only searches down.
func (c *Command) Find(args []string) (*Command, []string, error) {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  Find()") 
    fmt.Println("vendor/github.com/spf13/cobra/command.go  Find() c.Args :", c.Args)
	var innerfind func(*Command, []string) (*Command, []string)

	innerfind = func(c *Command, innerArgs []string) (*Command, []string) {
		argsWOflags := stripFlags(innerArgs, c)
		if len(argsWOflags) == 0 {
			return c, innerArgs
		}
		nextSubCmd := argsWOflags[0]

		cmd := c.findNext(nextSubCmd)
		if cmd != nil {
			return innerfind(cmd, argsMinusFirstX(innerArgs, nextSubCmd))
		}
		return c, innerArgs
	}

	commandFound, a := innerfind(c, args)
	if commandFound.Args == nil {
		return commandFound, a, legacyArgs(commandFound, stripFlags(a, commandFound))
	}
	return commandFound, a, nil
}

func (c *Command) findNext(next string) *Command {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  findNext()")
    fmt.Println("vendor/github.com/spf13/cobra/command.go  findNext() c.commands : ", c.commands)
    fmt.Println("vendor/github.com/spf13/cobra/command.go  findNext() args : ", next)
	matches := make([]*Command, 0)
	for _, cmd := range c.commands {
		if cmd.Name() == next || cmd.HasAlias(next) {
            fmt.Println("vendor/github.com/spf13/cobra/command.go  findNext() c.Args : ", cmd.Args)
			return cmd
		}
		if EnablePrefixMatching && cmd.HasNameOrAliasPrefix(next) {
			matches = append(matches, cmd)
		}
	}

	if len(matches) == 1 {
        fmt.Println("vendor/github.com/spf13/cobra/command.go  findNext() c.Args : ", matches[0].Args)
		return matches[0]
	}
	return nil
}

// Traverse the command tree to find the command, and parse args for
// each parent.
func (c *Command) Traverse(args []string) (*Command, []string, error) {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse()")
    fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse() c.Args :", c.Args)
	flags := []string{}
	inFlag := false

	for i, arg := range args {
        fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse() begin to switch()")
		switch {
		// A long flag with a space separated value
		case strings.HasPrefix(arg, "--") && !strings.Contains(arg, "="):
        fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse() Prefix--")
			// TODO: this isn't quite right, we should really check ahead for 'true' or 'false'
			inFlag = !isBooleanFlag(arg[2:], c.Flags())
			flags = append(flags, arg)
			continue
		// A short flag with a space separated value
		case strings.HasPrefix(arg, "-") && !strings.Contains(arg, "=") && len(arg) == 2 && !isBooleanShortFlag(arg[1:], c.Flags()):
        fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse() Prefix-")
			inFlag = true
			flags = append(flags, arg)
			continue
		// The value for a flag
		case inFlag:
        fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse() inFlag")
			inFlag = false
			flags = append(flags, arg)
			continue
		// A flag without a value, or with an `=` separated value
		case isFlagArg(arg):
        fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse() isFlagArg")
			flags = append(flags, arg)
			continue
		}
        fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse() switch()")

		cmd := c.findNext(arg)
		if cmd == nil {
            fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse() cmd is nil")
			return c, args, nil
		}

		if err := c.ParseFlags(flags); err != nil {
            fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse() parseFlag is err")
			return nil, args, err
		}
        fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse() begin to Recursion")
		return cmd.Traverse(args[i+1:])
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  Traverse() no switch()")
	return c, args, nil
}

func (c *Command) findSuggestions(arg string) string {
	if c.DisableSuggestions {
		return ""
	}
	if c.SuggestionsMinimumDistance <= 0 {
		c.SuggestionsMinimumDistance = 2
	}
	suggestionsString := ""
	if suggestions := c.SuggestionsFor(arg); len(suggestions) > 0 {
		suggestionsString += "\n\nDid you mean this?\n"
		for _, s := range suggestions {
			suggestionsString += fmt.Sprintf("\t%v\n", s)
		}
	}
	return suggestionsString
}

func (c *Command) SuggestionsFor(typedName string) []string {
	suggestions := []string{}
	for _, cmd := range c.commands {
		if cmd.IsAvailableCommand() {
			levenshteinDistance := ld(typedName, cmd.Name(), true)
			suggestByLevenshtein := levenshteinDistance <= c.SuggestionsMinimumDistance
			suggestByPrefix := strings.HasPrefix(strings.ToLower(cmd.Name()), strings.ToLower(typedName))
			if suggestByLevenshtein || suggestByPrefix {
				suggestions = append(suggestions, cmd.Name())
			}
			for _, explicitSuggestion := range cmd.SuggestFor {
				if strings.EqualFold(typedName, explicitSuggestion) {
					suggestions = append(suggestions, cmd.Name())
				}
			}
		}
	}
	return suggestions
}

func (c *Command) VisitParents(fn func(*Command)) {
	var traverse func(*Command) *Command

	traverse = func(x *Command) *Command {
		if x != c {
			fn(x)
		}
		if x.HasParent() {
			return traverse(x.parent)
		}
		return x
	}
	traverse(c)
}

func (c *Command) Root() *Command {
	var findRoot func(*Command) *Command

	findRoot = func(x *Command) *Command {
		if x.HasParent() {
			return findRoot(x.parent)
		}
		return x
	}

	return findRoot(c)
}

// ArgsLenAtDash will return the length of f.Args at the moment when a -- was
// found during arg parsing. This allows your program to know which args were
// before the -- and which came after. (Description from
// https://godoc.org/github.com/spf13/pflag#FlagSet.ArgsLenAtDash).
func (c *Command) ArgsLenAtDash() int {
	return c.Flags().ArgsLenAtDash()
}

func (c *Command) execute(a []string) (err error) {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  execute() ")

	if c == nil {
		return fmt.Errorf("Called Execute() on a nil Command")
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  execute() c.Args : ", c.Args)

	if len(c.Deprecated) > 0 {
		c.Printf("Command %q is deprecated, %s\n", c.Name(), c.Deprecated)
	}

	// initialize help flag as the last point possible to allow for user
	// overriding
	c.initHelpFlag()

	err = c.ParseFlags(a)
	if err != nil {
		return c.FlagErrorFunc()(c, err)
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  execute() parse flags")

	// If help is called, regardless of other flags, return we want help
	// Also say we need help if the command isn't runnable.
	helpVal, err := c.Flags().GetBool("help")
	if err != nil {
		// should be impossible to get here as we always declare a help
		// flag in initHelpFlag()
		c.Println("\"help\" flag declared as non-bool. Please correct your code")
        fmt.Println("vendor/github.com/spf13/cobra/command.go  execute() args help")
		return err
	}
	if helpVal || !c.Runnable() {
		return flag.ErrHelp
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  execute() args after help")

	c.preRun()
    fmt.Println("vendor/github.com/spf13/cobra/command.go  execute()  after preRun")

	argWoFlags := c.Flags().Args()
	if c.DisableFlagParsing {
		argWoFlags = a
	} 
    fmt.Println("vendor/github.com/spf13/cobra/command.go  execute() before validate args")

	if err := c.ValidateArgs(argWoFlags); err != nil {
		return err
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  execute() after validate args")

	for p := c; p != nil; p = p.Parent() {
		if p.PersistentPreRunE != nil {
			if err := p.PersistentPreRunE(c, argWoFlags); err != nil {
				return err
			}
			break
		} else if p.PersistentPreRun != nil {
			p.PersistentPreRun(c, argWoFlags)
			break
		}
	}
	if c.PreRunE != nil {
		if err := c.PreRunE(c, argWoFlags); err != nil {
			return err
		}
	} else if c.PreRun != nil {
		c.PreRun(c, argWoFlags)
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  execute() preRun")

	if c.RunE != nil {
		if err := c.RunE(c, argWoFlags); err != nil {
			return err
		}
	} else {
		c.Run(c, argWoFlags)
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  execute() RunE")

	if c.PostRunE != nil {
		if err := c.PostRunE(c, argWoFlags); err != nil {
			return err
		}
	} else if c.PostRun != nil {
		c.PostRun(c, argWoFlags)
	}
	for p := c; p != nil; p = p.Parent() {
		if p.PersistentPostRunE != nil {
			if err := p.PersistentPostRunE(c, argWoFlags); err != nil {
				return err
			}
			break
		} else if p.PersistentPostRun != nil {
			p.PersistentPostRun(c, argWoFlags)
			break
		}
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  execute() postRun")

	return nil
}

func (c *Command) preRun() {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  preRun()")
	for _, x := range initializers {
		x()
	}
}

func (c *Command) errorMsgFromParse() string {
	s := c.flagErrorBuf.String()

	x := strings.Split(s, "\n")

	if len(x) > 0 {
		return x[0]
	}
	return ""
}

// Call execute to use the args (os.Args[1:] by default)
// and run through the command tree finding appropriate matches
// for commands and then corresponding flags.
func (c *Command) Execute() error {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  Execute()")
	_, err := c.ExecuteC()
	return err
}

func (c *Command) ExecuteInFirstContainer() error {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteInFirstContainer()")
	_, err := c.ExecuteCmdInFirstContainer()
	return err
}

func (c *Command) ExecuteCmdInFirstContainer() (cmd *Command, err error) {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer()") 
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() c.Args : ", c.Args) 

	// Regardless of what command execute is called on, run on Root only
	if c.HasParent() {
		return c.Root().ExecuteC()
	}

	// windows hook
	if preExecHookFn != nil {
		preExecHookFn(c)
	}

	// initialize help as the last point possible to allow for user
	// overriding
	c.initHelpCmd()

	var args []string

	// Workaround FAIL with "go test -v" or "cobra.test -test.v", see #155
/*	if c.args == nil && filepath.Base(os.Args[0]) != "cobra.test" {
		args = os.Args[1:]
	} else {
		args = c.args
	}
*/
    
    if c.args != nil {
       args = c.args 
       fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() args :", args) 
    }else { 
       fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() args is nil") 
    }

    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() cmdArgs :", c.args) 
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() osArgs :???", os.Args) 
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() args :", args) 

	var flags []string
	if c.TraverseChildren {
        fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() cmd Traverse") 
		cmd, flags, err = c.Traverse(args)
	} else {
        fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() cmd Find") 
		cmd, flags, err = c.Find(args)
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() cmd : ", cmd) 
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() flags : ", flags)
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() err : ", err)

	if err != nil {
		// If found parse to a subcommand and then failed, talk about the subcommand
		if cmd != nil {
			c = cmd
		}
		if !c.SilenceErrors {
			c.Println("Error:", err.Error())
			c.Printf("Run '%v --help' for usage.\n", c.CommandPath())
		}
		return c, err
	}

    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() cmd flags : ", flags)
    var tmpSlice = []string{}
    for i := 0; i < len(flags); i++ {
         if i == 0 {
             fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() cmd flags[0] : ", flags[i])
             splitStringPrefix := strings.Fields(flags[i])
             fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() cmd prefix : ", len(splitStringPrefix))
             for j := 0; j < len(splitStringPrefix); j++ {
                 fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() cmd prefix : ", splitStringPrefix[j])
                 if j == 0 {
                    continue
                 }else {
                    tmpSlice = append(tmpSlice, splitStringPrefix[j])
                 }
             }
             continue
         }
         tmpSlice = append(tmpSlice, flags[i])
    }
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() cmd flags[1:] : ", tmpSlice)
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() cmd Args : ", cmd.Args)
	err = cmd.execute(tmpSlice)
    //err = cmd.execute(flags)
    //err = cmd.execute(flags[1:])
	if err != nil {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteCmdInFirstContainer() cmd exec is err :", err) 
		// Always show help if requested, even if SilenceErrors is in
		// effect
		if err == flag.ErrHelp {
			cmd.HelpFunc()(cmd, args)
			return cmd, nil
		}

		// If root command has SilentErrors flagged,
		// all subcommands should respect it
		if !cmd.SilenceErrors && !c.SilenceErrors {
			c.Println("Error:", err.Error())
		}

		// If root command has SilentUsage flagged,
		// all subcommands should respect it
		if !cmd.SilenceUsage && !c.SilenceUsage {
			c.Println(cmd.UsageString())
		}
		return cmd, err
	}
	return cmd, nil
}

func (c *Command) ExecuteC() (cmd *Command, err error) {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC()") 
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() c.Args : ", c.Args) 

	// Regardless of what command execute is called on, run on Root only
	if c.HasParent() {
		return c.Root().ExecuteC()
	}

	// windows hook
	if preExecHookFn != nil {
		preExecHookFn(c)
	}

	// initialize help as the last point possible to allow for user
	// overriding
	c.initHelpCmd()

	var args []string

	// Workaround FAIL with "go test -v" or "cobra.test -test.v", see #155
	if c.args == nil && filepath.Base(os.Args[0]) != "cobra.test" {
		args = os.Args[1:]
	} else {
		args = c.args
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() cmdArgs :", c.args) 
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() osArgs :", os.Args) 
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() args :", args)
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() c.Args :", c.Args)

	var flags []string
	if c.TraverseChildren {
        fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() cmd Traverse") 
		cmd, flags, err = c.Traverse(args)
	} else {
        fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() cmd Find") 
		cmd, flags, err = c.Find(args)
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() cmd : ", cmd) 
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() flags : ", flags)
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() err : ", err)

	if err != nil {
		// If found parse to a subcommand and then failed, talk about the subcommand
		if cmd != nil {
			c = cmd
		}
		if !c.SilenceErrors {
			c.Println("Error:", err.Error())
			c.Printf("Run '%v --help' for usage.\n", c.CommandPath())
		}
		return c, err
	}

    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() cmd flags : ", flags)
/*    var tmpSlice = []string{}
    for i := 0; i < len(flags); i++ {
         if i == 0 {
             fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() cmd flags[0] : ", flags[i])
             splitStringPrefix := strings.Fields(flags[i])
             fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() cmd prefix : ", len(splitStringPrefix))
             for j := 0; j < len(splitStringPrefix); j++ {
                 fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() cmd prefix : ", splitStringPrefix[j])
                 if j == 0 {
                    continue
                 }else {
                    tmpSlice = append(tmpSlice, splitStringPrefix[j])
                 }
             }
             continue
         }
         tmpSlice = append(tmpSlice, flags[i])
    }
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() cmd flags[1:] : ", tmpSlice)
*/
    //err = cmd.execute(tmpSlice)
    //err = cmd.execute(flags[1:])
    err = cmd.execute(flags)
	if err != nil {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ExecuteC() cmd exec is err :", err) 
		// Always show help if requested, even if SilenceErrors is in
		// effect
		if err == flag.ErrHelp {
			cmd.HelpFunc()(cmd, args)
			return cmd, nil
		}

		// If root command has SilentErrors flagged,
		// all subcommands should respect it
		if !cmd.SilenceErrors && !c.SilenceErrors {
			c.Println("Error:", err.Error())
		}

		// If root command has SilentUsage flagged,
		// all subcommands should respect it
		if !cmd.SilenceUsage && !c.SilenceUsage {
			c.Println(cmd.UsageString())
		}
		return cmd, err
	}
	return cmd, nil
}

func (c *Command) ValidateArgs(args []string) error {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ValidateArgs()")
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ValidateArgs() c.Args : ", c.Args)
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ValidateArgs() &c.Args : ", &c.Args)
	if c.Args == nil {
		return nil
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  ValidateArgs() c.Args != nil")
	return c.Args(c, args)
}

func (c *Command) initHelpFlag() {
	c.mergePersistentFlags()
	if c.Flags().Lookup("help") == nil {
		c.Flags().BoolP("help", "h", false, "help for "+c.Name())
	}
}

func (c *Command) initHelpCmd() {
	if c.helpCommand == nil {
		if !c.HasSubCommands() {
			return
		}

		c.helpCommand = &Command{
			Use:   "help [command]",
			Short: "Help about any command",
			Long: `Help provides help for any command in the application.
    Simply type ` + c.Name() + ` help [path to command] for full details.`,
			PersistentPreRun:  func(cmd *Command, args []string) {},
			PersistentPostRun: func(cmd *Command, args []string) {},

			Run: func(c *Command, args []string) {
				cmd, _, e := c.Root().Find(args)
				if cmd == nil || e != nil {
					c.Printf("Unknown help topic %#q.", args)
					c.Root().Usage()
				} else {
					helpFunc := cmd.HelpFunc()
					helpFunc(cmd, args)
				}
			},
		}
	}
	c.AddCommand(c.helpCommand)
}

// Used for testing
func (c *Command) ResetCommands() {
	c.commands = nil
	c.helpCommand = nil
}

// Sorts commands by their names
type commandSorterByName []*Command

func (c commandSorterByName) Len() int           { return len(c) }
func (c commandSorterByName) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c commandSorterByName) Less(i, j int) bool { return c[i].Name() < c[j].Name() }

// Commands returns a sorted slice of child commands.
func (c *Command) Commands() []*Command {
	// do not sort commands if it already sorted or sorting was disabled
	if EnableCommandSorting && !c.commandsAreSorted {
		sort.Sort(commandSorterByName(c.commands))
		c.commandsAreSorted = true
	}
	return c.commands
}

// AddCommand adds one or more commands to this parent command.
func (c *Command) AddCommand(cmds ...*Command) {
	for i, x := range cmds {
		if cmds[i] == c {
			panic("Command can't be a child of itself")
		}
		cmds[i].parent = c
		// update max lengths
		usageLen := len(x.Use)
		if usageLen > c.commandsMaxUseLen {
			c.commandsMaxUseLen = usageLen
		}
		commandPathLen := len(x.CommandPath())
		if commandPathLen > c.commandsMaxCommandPathLen {
			c.commandsMaxCommandPathLen = commandPathLen
		}
		nameLen := len(x.Name())
		if nameLen > c.commandsMaxNameLen {
			c.commandsMaxNameLen = nameLen
		}
		// If global normalization function exists, update all children
		if c.globNormFunc != nil {
			x.SetGlobalNormalizationFunc(c.globNormFunc)
		}
		c.commands = append(c.commands, x)
		c.commandsAreSorted = false
	}
}

// RemoveCommand removes one or more commands from a parent command.
func (c *Command) RemoveCommand(cmds ...*Command) {
	commands := []*Command{}
main:
	for _, command := range c.commands {
		for _, cmd := range cmds {
			if command == cmd {
				command.parent = nil
				continue main
			}
		}
		commands = append(commands, command)
	}
	c.commands = commands
	// recompute all lengths
	c.commandsMaxUseLen = 0
	c.commandsMaxCommandPathLen = 0
	c.commandsMaxNameLen = 0
	for _, command := range c.commands {
		usageLen := len(command.Use)
		if usageLen > c.commandsMaxUseLen {
			c.commandsMaxUseLen = usageLen
		}
		commandPathLen := len(command.CommandPath())
		if commandPathLen > c.commandsMaxCommandPathLen {
			c.commandsMaxCommandPathLen = commandPathLen
		}
		nameLen := len(command.Name())
		if nameLen > c.commandsMaxNameLen {
			c.commandsMaxNameLen = nameLen
		}
	}
}

// Print is a convenience method to Print to the defined output
func (c *Command) Print(i ...interface{}) {
	fmt.Fprint(c.Out(), i...)
}

// Println is a convenience method to Println to the defined output
func (c *Command) Println(i ...interface{}) {
	str := fmt.Sprintln(i...)
	c.Print(str)
}

// Printf is a convenience method to Printf to the defined output
func (c *Command) Printf(format string, i ...interface{}) {
	str := fmt.Sprintf(format, i...)
	c.Print(str)
}

// Output the usage for the command
// Used when a user provides invalid input
// Can be defined by user by overriding UsageFunc
func (c *Command) Usage() error {
	c.mergePersistentFlags()
	err := c.UsageFunc()(c)
	return err
}

// Output the help for the command
// Used when a user calls help [command]
// by the default HelpFunc in the commander
func (c *Command) Help() error {
	c.mergePersistentFlags()
	err := tmpl(c.getOutOrStdout(), c.HelpTemplate(), c)
	return err
}

func (c *Command) UsageString() string {
	tmpOutput := c.output
	bb := new(bytes.Buffer)
	c.SetOutput(bb)
	c.Usage()
	c.output = tmpOutput
	return bb.String()
}

// CommandPath returns the full path to this command.
func (c *Command) CommandPath() string {
	str := c.Name()
	x := c
	for x.HasParent() {
		str = x.parent.Name() + " " + str
		x = x.parent
	}
	return str
}

//The full usage for a given command (including parents)
func (c *Command) UseLine() string {
	str := ""
	if c.HasParent() {
		str = c.parent.CommandPath() + " "
	}
	return str + c.Use
}

// For use in determining which flags have been assigned to which commands
// and which persist
func (c *Command) DebugFlags() {
	c.Println("DebugFlags called on", c.Name())
	var debugflags func(*Command)

	debugflags = func(x *Command) {
		if x.HasFlags() || x.HasPersistentFlags() {
			c.Println(x.Name())
		}
		if x.HasFlags() {
			x.flags.VisitAll(func(f *flag.Flag) {
				if x.HasPersistentFlags() {
					if x.persistentFlag(f.Name) == nil {
						c.Println("  -"+f.Shorthand+",", "--"+f.Name, "["+f.DefValue+"]", "", f.Value, "  [L]")
					} else {
						c.Println("  -"+f.Shorthand+",", "--"+f.Name, "["+f.DefValue+"]", "", f.Value, "  [LP]")
					}
				} else {
					c.Println("  -"+f.Shorthand+",", "--"+f.Name, "["+f.DefValue+"]", "", f.Value, "  [L]")
				}
			})
		}
		if x.HasPersistentFlags() {
			x.pflags.VisitAll(func(f *flag.Flag) {
				if x.HasFlags() {
					if x.flags.Lookup(f.Name) == nil {
						c.Println("  -"+f.Shorthand+",", "--"+f.Name, "["+f.DefValue+"]", "", f.Value, "  [P]")
					}
				} else {
					c.Println("  -"+f.Shorthand+",", "--"+f.Name, "["+f.DefValue+"]", "", f.Value, "  [P]")
				}
			})
		}
		c.Println(x.flagErrorBuf)
		if x.HasSubCommands() {
			for _, y := range x.commands {
				debugflags(y)
			}
		}
	}

	debugflags(c)
}

// Name returns the command's name: the first word in the use line.
func (c *Command) Name() string {
	if c.name != "" {
		return c.name
	}
	name := c.Use
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

// HasAlias determines if a given string is an alias of the command.
func (c *Command) HasAlias(s string) bool {
	for _, a := range c.Aliases {
		if a == s {
			return true
		}
	}
	return false
}

// HasNameOrAliasPrefix returns true if the Name or any of aliases start
// with prefix
func (c *Command) HasNameOrAliasPrefix(prefix string) bool {
	if strings.HasPrefix(c.Name(), prefix) {
		return true
	}
	for _, alias := range c.Aliases {
		if strings.HasPrefix(alias, prefix) {
			return true
		}
	}
	return false
}

func (c *Command) NameAndAliases() string {
	return strings.Join(append([]string{c.Name()}, c.Aliases...), ", ")
}

func (c *Command) HasExample() bool {
	return len(c.Example) > 0
}

// Runnable determines if the command is itself runnable
func (c *Command) Runnable() bool {
	return c.Run != nil || c.RunE != nil
}

// HasSubCommands determines if the command has children commands
func (c *Command) HasSubCommands() bool {
	return len(c.commands) > 0
}

// IsAvailableCommand determines if a command is available as a non-help command
// (this includes all non deprecated/hidden commands)
func (c *Command) IsAvailableCommand() bool {
	if len(c.Deprecated) != 0 || c.Hidden {
		return false
	}

	if c.HasParent() && c.Parent().helpCommand == c {
		return false
	}

	if c.Runnable() || c.HasAvailableSubCommands() {
		return true
	}

	return false
}

// IsHelpCommand determines if a command is a 'help' command; a help command is
// determined by the fact that it is NOT runnable/hidden/deprecated, and has no
// sub commands that are runnable/hidden/deprecated
func (c *Command) IsHelpCommand() bool {

	// if a command is runnable, deprecated, or hidden it is not a 'help' command
	if c.Runnable() || len(c.Deprecated) != 0 || c.Hidden {
		return false
	}

	// if any non-help sub commands are found, the command is not a 'help' command
	for _, sub := range c.commands {
		if !sub.IsHelpCommand() {
			return false
		}
	}

	// the command either has no sub commands, or no non-help sub commands
	return true
}

// HasHelpSubCommands determines if a command has any avilable 'help' sub commands
// that need to be shown in the usage/help default template under 'additional help
// topics'
func (c *Command) HasHelpSubCommands() bool {

	// return true on the first found available 'help' sub command
	for _, sub := range c.commands {
		if sub.IsHelpCommand() {
			return true
		}
	}

	// the command either has no sub commands, or no available 'help' sub commands
	return false
}

// HasAvailableSubCommands determines if a command has available sub commands that
// need to be shown in the usage/help default template under 'available commands'
func (c *Command) HasAvailableSubCommands() bool {

	// return true on the first found available (non deprecated/help/hidden)
	// sub command
	for _, sub := range c.commands {
		if sub.IsAvailableCommand() {
			return true
		}
	}

	// the command either has no sub comamnds, or no available (non deprecated/help/hidden)
	// sub commands
	return false
}

// Determine if the command is a child command
func (c *Command) HasParent() bool {
	return c.parent != nil
}

// GlobalNormalizationFunc returns the global normalization function or nil if doesn't exists
func (c *Command) GlobalNormalizationFunc() func(f *flag.FlagSet, name string) flag.NormalizedName {
	return c.globNormFunc
}

// Get the complete FlagSet that applies to this command (local and persistent declared here and by all parents)
func (c *Command) Flags() *flag.FlagSet {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  Flags()")
	if c.flags == nil {
		c.flags = flag.NewFlagSet(c.Name(), flag.ContinueOnError)
		if c.flagErrorBuf == nil {
			c.flagErrorBuf = new(bytes.Buffer)
		}
		c.flags.SetOutput(c.flagErrorBuf)
	}
    fmt.Println("vendor/github.com/spf13/cobra/command.go  Flags() c.Args : ", c.Args)
	return c.flags
}

// LocalNonPersistentFlags are flags specific to this command which will NOT persist to subcommands
func (c *Command) LocalNonPersistentFlags() *flag.FlagSet {
	persistentFlags := c.PersistentFlags()

	out := flag.NewFlagSet(c.Name(), flag.ContinueOnError)
	c.LocalFlags().VisitAll(func(f *flag.Flag) {
		if persistentFlags.Lookup(f.Name) == nil {
			out.AddFlag(f)
		}
	})
	return out
}

// Get the local FlagSet specifically set in the current command
func (c *Command) LocalFlags() *flag.FlagSet {
	c.mergePersistentFlags()

	local := flag.NewFlagSet(c.Name(), flag.ContinueOnError)
	c.lflags.VisitAll(func(f *flag.Flag) {
		local.AddFlag(f)
	})
	if !c.HasParent() {
		flag.CommandLine.VisitAll(func(f *flag.Flag) {
			if local.Lookup(f.Name) == nil {
				local.AddFlag(f)
			}
		})
	}
	return local
}

// All Flags which were inherited from parents commands
func (c *Command) InheritedFlags() *flag.FlagSet {
	c.mergePersistentFlags()

	inherited := flag.NewFlagSet(c.Name(), flag.ContinueOnError)
	local := c.LocalFlags()

	var rmerge func(x *Command)

	rmerge = func(x *Command) {
		if x.HasPersistentFlags() {
			x.PersistentFlags().VisitAll(func(f *flag.Flag) {
				if inherited.Lookup(f.Name) == nil && local.Lookup(f.Name) == nil {
					inherited.AddFlag(f)
				}
			})
		}
		if x.HasParent() {
			rmerge(x.parent)
		}
	}

	if c.HasParent() {
		rmerge(c.parent)
	}

	return inherited
}

// All Flags which were not inherited from parent commands
func (c *Command) NonInheritedFlags() *flag.FlagSet {
	return c.LocalFlags()
}

// Get the Persistent FlagSet specifically set in the current command
func (c *Command) PersistentFlags() *flag.FlagSet {
	if c.pflags == nil {
		c.pflags = flag.NewFlagSet(c.Name(), flag.ContinueOnError)
		if c.flagErrorBuf == nil {
			c.flagErrorBuf = new(bytes.Buffer)
		}
		c.pflags.SetOutput(c.flagErrorBuf)
	}
	return c.pflags
}

// For use in testing
func (c *Command) ResetFlags() {
	c.flagErrorBuf = new(bytes.Buffer)
	c.flagErrorBuf.Reset()
	c.flags = flag.NewFlagSet(c.Name(), flag.ContinueOnError)
	c.flags.SetOutput(c.flagErrorBuf)
	c.pflags = flag.NewFlagSet(c.Name(), flag.ContinueOnError)
	c.pflags.SetOutput(c.flagErrorBuf)
}

// Does the command contain any flags (local plus persistent from the entire structure)
func (c *Command) HasFlags() bool {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  HasFlags()")
	return c.Flags().HasFlags()
}

// Does the command contain persistent flags
func (c *Command) HasPersistentFlags() bool {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  HasPersistentFlags()")
	return c.PersistentFlags().HasFlags()
}

// Does the command has flags specifically declared locally
func (c *Command) HasLocalFlags() bool {
	return c.LocalFlags().HasFlags()
}

// Does the command have flags inherited from its parent command
func (c *Command) HasInheritedFlags() bool {
	return c.InheritedFlags().HasFlags()
}

// Does the command contain any flags (local plus persistent from the entire
// structure) which are not hidden or deprecated
func (c *Command) HasAvailableFlags() bool {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  HasAvailableFlags()")
	return c.Flags().HasAvailableFlags()
}

// Does the command contain persistent flags which are not hidden or deprecated
func (c *Command) HasAvailablePersistentFlags() bool {
	return c.PersistentFlags().HasAvailableFlags()
}

// Does the command has flags specifically declared locally which are not hidden
// or deprecated
func (c *Command) HasAvailableLocalFlags() bool {
	return c.LocalFlags().HasAvailableFlags()
}

// Does the command have flags inherited from its parent command which are
// not hidden or deprecated
func (c *Command) HasAvailableInheritedFlags() bool {
	return c.InheritedFlags().HasAvailableFlags()
}

// Flag climbs up the command tree looking for matching flag
func (c *Command) Flag(name string) (flag *flag.Flag) {
    fmt.Println("vendor/github.com/spf13/cobra/command.go  Flag()")
	flag = c.Flags().Lookup(name)

	if flag == nil {
		flag = c.persistentFlag(name)
	}

	return
}

// recursively find matching persistent flag
func (c *Command) persistentFlag(name string) (flag *flag.Flag) {
	if c.HasPersistentFlags() {
		flag = c.PersistentFlags().Lookup(name)
	}

	if flag == nil && c.HasParent() {
		flag = c.parent.persistentFlag(name)
	}
	return
}

// ParseFlags parses persistent flag tree & local flags
func (c *Command) ParseFlags(args []string) (err error) {
	if c.DisableFlagParsing {
		return nil
	}
	c.mergePersistentFlags()
	err = c.Flags().Parse(args)
	return
}

// Parent returns a commands parent command
func (c *Command) Parent() *Command {
	return c.parent
}

func (c *Command) mergePersistentFlags() {
	var rmerge func(x *Command)

	// Save the set of local flags
	if c.lflags == nil {
		c.lflags = flag.NewFlagSet(c.Name(), flag.ContinueOnError)
		if c.flagErrorBuf == nil {
			c.flagErrorBuf = new(bytes.Buffer)
		}
		c.lflags.SetOutput(c.flagErrorBuf)
		addtolocal := func(f *flag.Flag) {
			c.lflags.AddFlag(f)
		}
        fmt.Println("vendor/github.com/spf13/cobra/command.go  mergePersistentFlags()")
		c.Flags().VisitAll(addtolocal)
		c.PersistentFlags().VisitAll(addtolocal)
	}
	rmerge = func(x *Command) {
		if !x.HasParent() {
			flag.CommandLine.VisitAll(func(f *flag.Flag) {
				if x.PersistentFlags().Lookup(f.Name) == nil {
					x.PersistentFlags().AddFlag(f)
				}
			})
		}
		if x.HasPersistentFlags() {
			x.PersistentFlags().VisitAll(func(f *flag.Flag) {
                fmt.Println("vendor/github.com/spf13/cobra/command.go  mergePersistentFlags() visitall")
				if c.Flags().Lookup(f.Name) == nil {
					c.Flags().AddFlag(f)
				}
			})
		}
		if x.HasParent() {
			rmerge(x.parent)
		}
	}

	rmerge(c)
}
