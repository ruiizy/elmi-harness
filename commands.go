package main

import (
	"fmt"
	"sort"
	"strings"
)

type command struct {
	description string
	usage       string
	run         func(args string)
}

func runCommand(registry map[string]command, line string) bool {
	if !strings.HasPrefix(line, "/") {
		return false
	}
	parts := strings.SplitN(strings.TrimPrefix(line, "/"), " ", 2)
	name, args := parts[0], ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	c, ok := registry[name]
	if !ok {
		fmt.Printf("unknown command: /%s  (try /help)\n", name)
		return true
	}
	c.run(args)
	return true
}

func cmdHelp(registry map[string]command, _ string) {
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	sort.Strings(names)
	fmt.Println("commands:")
	for _, n := range names {
		c := registry[n]
		if c.usage != "" {
			fmt.Printf("  /%-12s %s  (%s)\n", n, c.description, c.usage)
		} else {
			fmt.Printf("  /%-12s %s\n", n, c.description)
		}
	}
}
