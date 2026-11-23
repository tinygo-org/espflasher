package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

type command struct {
	name    string
	aliases []string
	args    string
	summary string
	run     func(args []string) error
}

var commands []*command

func init() {
	commands = []*command{
		{name: "write-flash", args: "<offset> <file> [<offset> <file>...]", summary: "Write binary files to flash", run: runWriteFlash},
		{name: "read-flash", args: "<offset> <size|ALL> <file>", summary: "Read flash contents to a file", run: runReadFlash},
		{name: "verify-flash", args: "<offset> <file> [<offset> <file>...]", summary: "Compare flash contents with files using MD5", run: runVerifyFlash},
		{name: "erase-flash", summary: "Erase the entire flash", run: runEraseFlash},
		{name: "erase-region", args: "<offset> <size>", summary: "Erase a flash region", run: runEraseRegion},
		{name: "chip-id", summary: "Print chip type, revision, features and MAC", run: runChipID},
		{name: "flash-id", summary: "Print SPI flash manufacturer, device and size", run: runFlashID},
		{name: "read-mac", summary: "Print the base MAC address", run: runReadMAC},
		{name: "get-security-info", summary: "Print security info from the ROM loader", run: runSecurityInfo},
		{name: "read-mem", aliases: []string{"read-reg"}, args: "<address>", summary: "Read a 32 bit word from memory", run: runReadMem},
		{name: "write-mem", aliases: []string{"write-reg"}, args: "<address> <value> [mask]", summary: "Write a 32 bit word to memory", run: runWriteMem},
		{name: "run", summary: "Reset the chip to run the application", run: runRun},
		{name: "version", summary: "Print version", run: runVersion},
		{name: "help", args: "[command]", summary: "Show help for a command", run: runHelp},
	}
}

func lookupCommand(name string) *command {
	name = strings.ReplaceAll(strings.ToLower(name), "_", "-")
	for _, c := range commands {
		if c.name == name || slices.Contains(c.aliases, name) {
			return c
		}
	}
	return nil
}

// newFlagSet returns a FlagSet with usage text for cmd.
func newFlagSet(cmd *command) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd.name, flag.ContinueOnError)
	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintf(w, "Usage: %s %s [options] %s\n\n%s\n", progName(), cmd.name, cmd.args, cmd.summary)
		if len(cmd.aliases) > 0 {
			fmt.Fprintf(w, "Aliases: %s\n", strings.Join(cmd.aliases, ", "))
		}
		fmt.Fprintln(w, "\nOptions:")
		fs.PrintDefaults()
	}
	return fs
}

// parseFlags parses args and checks the positional arg count.
func parseFlags(fs *flag.FlagSet, args []string, minArgs, maxArgs int) ([]string, error) {
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return nil, err
	}
	n := fs.NArg()
	if n < minArgs || (maxArgs >= 0 && n > maxArgs) {
		fs.Usage()
		return nil, errUsage
	}
	return fs.Args(), nil
}

var errUsage = fmt.Errorf("invalid arguments")

func progName() string {
	return "espflasher"
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s <command> [options] [args]\n\n", progName())
	fmt.Fprintln(w, "Flash and inspect Espressif ESP8266 and ESP32 family devices.")
	fmt.Fprintln(w, "\nCommands:")
	for _, c := range commands {
		fmt.Fprintf(w, "  %-18s %s\n", c.name, c.summary)
	}
	fmt.Fprintln(w, "\nUnderscore names like write_flash also work.")
	fmt.Fprintf(w, "Run '%s help <command>' for command options.\n", progName())
	fmt.Fprintf(w, "\nWithout a command, the old usage still works:\n  %s -port /dev/ttyUSB0 firmware.bin\n", progName())
}

func runHelp(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return nil
	}
	c := lookupCommand(args[0])
	if c == nil {
		return fmt.Errorf("unknown command: %s", args[0])
	}
	return c.run([]string{"-h"})
}
