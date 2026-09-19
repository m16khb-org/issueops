package basiccli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	issueopscontract "issueops/internal/contract/issueops"
	tracecontract "issueops/internal/contract/trace"
	"os"
)

func runTrace(args []string) error {
	if len(args) == 0 {
		traceUsage()
		return fmt.Errorf("missing trace subcommand")
	}
	switch args[0] {
	case "analyze":
		return runTraceAnalyze(args[1:])
	case "handoff-delivery":
		return runTraceHandoffDeliveryObserve(args[1:])
	default:
		traceUsage()
		return fmt.Errorf("unknown trace subcommand %q", args[0])
	}
}

func traceUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  issueops trace analyze --input <jsonl|state-key> [--json]
  issueops trace handoff-delivery --input <observation.json|-> [--json]
`)
}

func runTraceHandoffDeliveryObserve(args []string) error {
	fs := flag.NewFlagSet("trace handoff-delivery", flag.ContinueOnError)
	input := fs.String("input", "", "delivery observation JSON path or '-' for stdin")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" && fs.NArg() > 0 {
		*input = fs.Arg(0)
	}
	if *input == "" || TraceHandoffDeliveryObserve == nil {
		return fmt.Errorf("handoff delivery observation input and adapter are required")
	}
	var reader io.Reader
	if *input == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(*input)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, 256*1024+1))
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > 256*1024 {
		return fmt.Errorf("handoff delivery observation input is empty or oversized")
	}
	var observation issueopscontract.IssueOpsHandoffDeliveryObservation
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&observation); err != nil {
		return fmt.Errorf("decode handoff delivery observation: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("handoff delivery observation contains trailing JSON")
	}
	result, err := TraceHandoffDeliveryObserve(observation)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("handoff delivery observation: %s receipt=%s\n", result.AuditLogID, result.Observation.Receipt.Location)
	return nil
}

func runTraceAnalyze(args []string) error {
	fs := flag.NewFlagSet("trace analyze", flag.ContinueOnError)
	input := fs.String("input", "", "trace input path, '-' for stdin, or issueops state key")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" && fs.NArg() > 0 {
		*input = fs.Arg(0)
	}
	result, err := TraceAnalyze(tracecontract.TraceAnalyzeRequest{Input: *input})
	if err != nil {
		if *jsonOut {
			_ = printJSON(result)
		} else {
			traceUsage()
		}
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("trace analysis: %d finding(s) from %s\n", result.FindingCount, result.InputSource)
	for _, finding := range result.Findings {
		fmt.Printf("- %s: %s\n", finding.FailureClass, finding.RecurringPattern)
		fmt.Printf("  knob: %s\n", finding.ProposedKnob)
		fmt.Printf("  verify: %s\n", finding.VerificationCommand)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return nil
}
