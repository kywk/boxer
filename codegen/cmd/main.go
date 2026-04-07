package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/boxer/codegen/ir"
	"github.com/boxer/codegen/targets/golang"
)

func main() {
	target := flag.String("target", "golang", "codegen target: golang")
	input := flag.String("input", "", "input IR JSON file (required)")
	output := flag.String("output", "", "output file (default: stdout)")
	flag.Parse()

	if *input == "" {
		fmt.Fprintln(os.Stderr, "usage: gateway-codegen -input flow.json [-target golang] [-output handler.go]")
		os.Exit(1)
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fatal("read input: %v", err)
	}

	var flow ir.GatewayIR
	if err := json.Unmarshal(data, &flow); err != nil {
		fatal("parse IR: %v", err)
	}

	switch *target {
	case "golang":
		result, err := golang.Generate(&flow)
		if err != nil {
			fatal("codegen: %v", err)
		}

		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
		fmt.Fprintf(os.Stderr, "prerequisites: %v\n", result.Prerequisites)

		if *output != "" {
			if err := os.WriteFile(*output, []byte(result.Code), 0644); err != nil {
				fatal("write output: %v", err)
			}
			fmt.Fprintf(os.Stderr, "wrote %s\n", *output)
		} else {
			fmt.Print(result.Code)
		}

	default:
		fatal("unknown target: %s", *target)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
