package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/boxer/codegen/ir"
	"github.com/boxer/codegen/targets/golang"
	"github.com/boxer/codegen/targets/kong"
)

func main() {
	target := flag.String("target", "golang", "codegen target: golang | kong")
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

	var code string
	var prereqs []string

	switch *target {
	case "golang":
		result, err := golang.Generate(&flow)
		if err != nil {
			fatal("codegen: %v", err)
		}
		code = result.Code
		prereqs = result.Prerequisites
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}

	case "kong":
		result, err := kong.Generate(&flow)
		if err != nil {
			fatal("codegen: %v", err)
		}
		code = result.Code
		prereqs = result.Prerequisites

	default:
		fatal("unknown target: %s", *target)
	}

	fmt.Fprintf(os.Stderr, "prerequisites: %v\n", prereqs)

	if *output != "" {
		if err := os.WriteFile(*output, []byte(code), 0644); err != nil {
			fatal("write output: %v", err)
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", *output)
	} else {
		fmt.Print(code)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
