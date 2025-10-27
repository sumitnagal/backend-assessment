package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "help":
		printHelp()
	case "version":
		fmt.Println("Backend Assessment CLI v1.0.0")
	case "worker-load":
		runWorkerLoad()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("Backend Assessment CLI")
	fmt.Println("\nUsage:")
	fmt.Println("  cli <command> [options]")
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  help     - Show this help message")
	fmt.Println("  version  - Show version information")
	fmt.Println("  worker-load - Run message worker under synthetic load")
}

func runWorkerLoad() {
	fs := flag.NewFlagSet("worker-load", flag.ExitOnError)
	var (
		ratePerSec  int
		durationSec int
		queueSize   int
		workerID    int
	)
	fs.IntVar(&ratePerSec, "rate", 10, "messages per second")
	fs.IntVar(&durationSec, "duration", 30, "test duration in seconds")
	fs.IntVar(&queueSize, "queue", 100, "worker queue size")
	fs.IntVar(&workerID, "worker", 1, "worker ID")
	_ = fs.Parse(os.Args[2:])

	// Defer implementation to separate file for clarity
	runWorkerLoadImpl(ratePerSec, durationSec, queueSize, workerID)
}
