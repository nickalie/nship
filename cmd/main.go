package main

import (
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	"os"

	"ngdeploy/config"
	"ngdeploy/pkg/job"
)

func main() {
	configPath := flag.String("config", "deploy.yaml", "Path to YAML configuration file")
	jobName := flag.String("job", "", "Name of specific job to run (runs all jobs if not specified)")
	envPath := flag.String("env", "", "Path to .env file")
	flag.Parse()

	if *envPath != "" {
		err := godotenv.Load(*envPath)
		if err != nil {
			fmt.Printf("Error loading .env file: %v\n", err)
			os.Exit(1)
		}
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	var jobsToRun []config.Job
	if *jobName != "" {
		for _, j := range cfg.Jobs {
			if j.Name == *jobName {
				jobsToRun = append(jobsToRun, j)
				break
			}
		}
		if len(jobsToRun) == 0 {
			fmt.Printf("Error: Job '%s' not found in configuration\n", *jobName)
			os.Exit(1)
		}
	} else {
		jobsToRun = cfg.Jobs
	}

	for _, target := range cfg.Targets {
		for _, j := range jobsToRun {
			fmt.Printf("Running job '%s' on target '%s'\n", j.Name, target.Name)
			err := job.RunJob(target, j)
			if err != nil {
				fmt.Printf("Error running job '%s' on target '%s': %v\n", j.Name, target.Name, err)
				continue
			}
			fmt.Printf("Job '%s' completed successfully on target '%s'\n", j.Name, target.Name)
		}
	}
}
