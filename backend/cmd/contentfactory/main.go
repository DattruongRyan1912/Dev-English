package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/contentfactory"
)

func main() {
	root := flag.String("root", ".", "repository root")
	writeCases := flag.String("write-evaluation-cases", "", "write a generated deterministic evaluation fixture")
	target := flag.Int("target", 200, "generated evaluation case count")
	flag.Parse()
	catalog, err := contentfactory.Load(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report := contentfactory.BuildReport(catalog)
	if *writeCases != "" {
		if err := contentfactory.WriteEvaluationCases(*writeCases, *target); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	encoded, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(encoded))
}
