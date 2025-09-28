package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const helpText = `ptracker - Personal Time Tracker CLI
Track time spent on your projects with simple commands.

USAGE:
  ptracker [COMMAND] [OPTIONS]

COMMANDS:
  create [project]       Create a new project
  delete [project]       Delete a project and all its logs
  start [project]        Start tracking time on a project
  stop [project]         Stop tracking the specified project
  status                 Show active tracking sessions
  stats [project]        View time log for a project
  report                 Show a summary of total time spent across all projects
  list                   List all tracked projects
  help                   Show this help message

EXAMPLES:
  ptracker create my_website
  ptracker start my_website
  ptracker stop my_website
  ptracker stats my_website
  ptracker report

NOTES:
- Time is automatically recorded using UTC.
- Multiple projects can have active sessions simultaneously.

Happy tracking.`

type LogEntry struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Project struct {
	Name      string        `json:"name"`
	Logs      []LogEntry    `json:"logs"`
	TotalTime time.Duration `json:"totalTime"`
}

type TrackerData struct {
	Projects []Project `json:"projects"`
}

func getAppPaths() (dataPath, logPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(home, ".ptracker")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "data.json"), filepath.Join(dir, "ptracker.log"), nil
}

func loadTracker(filename string) (*TrackerData, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &TrackerData{}, nil
		}
		return nil, err
	}
	var tracker TrackerData
	if err := json.Unmarshal(data, &tracker); err != nil {
		return nil, err
	}
	return &tracker, nil
}

func saveTracker(filename string, tracker *TrackerData) error {
	data, err := json.MarshalIndent(tracker, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func projectExists(tracker *TrackerData, name string) bool {
	for _, p := range tracker.Projects {
		if p.Name == name {
			return true
		}
	}
	return false
}

func main() {
	dataPath, logPath, err := getAppPaths()
	if err != nil {
		fmt.Println("Error resolving paths:", err)
		return
	}
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	args := os.Args
	now := time.Now().UTC()
	log.Println("Invoked:", args)

	if len(args) < 2 {
		fmt.Println("No command provided. Use 'help'.")
		return
	}

	tracker, err := loadTracker(dataPath)
	if err != nil {
		log.Fatal(err)
	}

	switch args[1] {
	case "help":
		fmt.Println(helpText)

	case "create":
		if len(args) < 3 {
			fmt.Println("Project name required.\n", helpText)
			return
		}
		name := args[2]
		if projectExists(tracker, name) {
			fmt.Printf("Project '%s' exists.\n", name)
			return
		}
		tracker.Projects = append(tracker.Projects, Project{Name: name})
		saveTracker(dataPath, tracker)
		fmt.Printf("Project '%s' created.\n", name)

	case "delete":
		if len(args) < 3 {
			fmt.Println("Project name required.\n", helpText)
			return
		}
		name := args[2]
		for i, p := range tracker.Projects {
			if p.Name == name {
				fmt.Printf("Delete '%s'? [y/N]: ", name)
				var r string
				fmt.Scanln(&r)
				if r != "y" && r != "Y" {
					fmt.Println("Cancelled.")
					return
				}
				tracker.Projects = append(tracker.Projects[:i], tracker.Projects[i+1:]...)
				saveTracker(dataPath, tracker)
				fmt.Printf("Deleted '%s'.\n", name)
				return
			}
		}
		fmt.Printf("'%s' not found.\n", name)

	case "start":
		if len(args) < 3 {
			fmt.Println("Project name required.\n", helpText)
			return
		}
		name := args[2]
		for i, p := range tracker.Projects {
			if p.Name == name {
				logs := p.Logs
				if len(logs) > 0 && logs[len(logs)-1].End.IsZero() {
					fmt.Println("Already active.")
					return
				}
				tracker.Projects[i].Logs = append(logs, LogEntry{Start: now})
				saveTracker(dataPath, tracker)
				fmt.Printf("Started '%s' at %s\n", name, now.Format(time.RFC822))
				return
			}
		}
		fmt.Printf("'%s' not found.\n", name)

	case "stop":
		if len(args) < 3 {
			fmt.Println("Project name required.\n", helpText)
			return
		}
		name := args[2]
		for i, p := range tracker.Projects {
			if p.Name == name {
				logs := p.Logs
				if len(logs) == 0 || !logs[len(logs)-1].End.IsZero() {
					fmt.Println("Not active.")
					return
				}
				end := now
				dur := end.Sub(logs[len(logs)-1].Start)
				tracker.Projects[i].Logs[len(logs)-1].End = end
				tracker.Projects[i].TotalTime += dur
				saveTracker(dataPath, tracker)
				fmt.Printf("Stopped '%s': %.2fmin (Total: %.2fmin)\n", name, dur.Minutes(), tracker.Projects[i].TotalTime.Minutes())
				return
			}
		}
		fmt.Printf("'%s' not found.\n", name)

	case "list":
		fmt.Println("Projects:")
		for _, p := range tracker.Projects {
			fmt.Println("- ", p.Name)
		}

	case "status":
		fmt.Println("Active Sessions:")
		count := 0
		for _, p := range tracker.Projects {
			if len(p.Logs) > 0 && p.Logs[len(p.Logs)-1].End.IsZero() {
				start := p.Logs[len(p.Logs)-1].Start
				dur := time.Since(start)
				fmt.Printf("* %-10s | Started: %s | Elapsed: %.2fmin\n", p.Name, start.Format("15:04:05"), dur.Minutes())
				count++
			}
		}
		if count == 0 {
			fmt.Println("None")
		}

	case "stats":
		if len(args) < 3 {
			fmt.Println("Project name required.\n", helpText)
			return
		}
		name := args[2]
		for _, p := range tracker.Projects {
			if p.Name == name {
				fmt.Println("===============================================")
				fmt.Printf("Stats for %s:\n", name)
				fmt.Println("===============================================")
				fmt.Printf("Total Sessions: %d | Total Time: %.2fmin\n", len(p.Logs), p.TotalTime.Minutes())
				if len(p.Logs) > 0 {
					fmt.Println("# | Start               | End                 | Duration(min)")
					fmt.Println("---|---------------------|---------------------|-------------")
					for i, e := range p.Logs {
						start := e.Start.Format("2006-01-02 15:04:05")
						end := "-"
						dur := time.Since(e.Start)
						if !e.End.IsZero() {
							end = e.End.Format("2006-01-02 15:04:05")
							dur = e.End.Sub(e.Start)
						}
						fmt.Printf("%-3d| %-20s| %-20s| %6.2f\n", i+1, start, end, dur.Minutes())
					}
				}
				return
			}
		}
		fmt.Printf("'%s' not found.\n", name)

	case "report":
		if len(tracker.Projects) == 0 {
			fmt.Println("No projects.")
			return
		}
		// compute grand total
		var totalAll time.Duration
		for _, p := range tracker.Projects {
			t := p.TotalTime
			if len(p.Logs) > 0 && p.Logs[len(p.Logs)-1].End.IsZero() {
				t += time.Since(p.Logs[len(p.Logs)-1].Start)
			}
			totalAll += t
		}
		fmt.Println("===================================================================")
		fmt.Println("Summary Report: All Projects")
		fmt.Println("===================================================================")
		fmt.Printf("%-16s | %-8s | %-10s | %-8s\n", "Project", "Sessions", "Time(min)", "Percent")
		fmt.Println("-----------------|----------|------------|--------")
		for _, p := range tracker.Projects {
			t := p.TotalTime
			if len(p.Logs) > 0 && p.Logs[len(p.Logs)-1].End.IsZero() {
				t += time.Since(p.Logs[len(p.Logs)-1].Start)
			}
			percent := 0.0
			if totalAll > 0 {
				percent = (t.Minutes() / totalAll.Minutes()) * 100
			}
			fmt.Printf("%-16s | %-8d | %-10.2f | %6.2f%%\n", p.Name, len(p.Logs), t.Minutes(), percent)
		}
		fmt.Println("-------------------------------------------------------------------")
		fmt.Printf("Total time tracked: %.2f minutes\n", totalAll.Minutes())

	default:
		fmt.Println("Unknown command. Use 'help'.")
	}
}
