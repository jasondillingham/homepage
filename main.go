package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Service struct {
	Name    string `json:"name"`
	PID     string `json:"pid"`
	Port    int    `json:"port"`
	Address string `json:"address"`
}

func getServices() ([]Service, error) {
	out, err := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-nP").Output()
	if err != nil {
		return nil, fmt.Errorf("lsof failed: %w", err)
	}

	var services []Service
	portRe := regexp.MustCompile(`:(\d+)\s+\(LISTEN\)`)
	addrRe := regexp.MustCompile(`\s([\d.*:[\]]+:\d+)\s+\(LISTEN\)`)

	lines := strings.Split(string(out), "\n")
	seen := make(map[string]bool)

	for _, line := range lines[1:] { // skip header
		if line == "" {
			continue
		}

		portMatch := portRe.FindStringSubmatch(line)
		if portMatch == nil {
			continue
		}

		port, _ := strconv.Atoi(portMatch[1])
		if (port < 8000 || port > 8999) && (port < 9000 || port > 9999) {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		pid := fields[1]

		addrMatch := addrRe.FindStringSubmatch(line)
		addr := fmt.Sprintf("*:%d", port)
		if addrMatch != nil {
			addr = addrMatch[1]
		}

		key := fmt.Sprintf("%s-%d", pid, port)
		if seen[key] {
			continue
		}
		seen[key] = true

		services = append(services, Service{
			Name:    name,
			PID:     pid,
			Port:    port,
			Address: addr,
		})
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Port < services[j].Port
	})

	return services, nil
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	services, err := getServices()
	if err != nil {
		log.Printf("Error getting services: %v", err)
	}
	tmpl.Execute(w, services)
}

func handleServices(w http.ResponseWriter, r *http.Request) {
	services, err := getServices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		tmpl := template.Must(template.ParseFiles("templates/partials/service_table.html"))
		tmpl.Execute(w, services)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(services)
}

// getProcessCommand returns the full command line for a given PID.
func getProcessCommand(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return "", fmt.Errorf("ps failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// stopProcess sends SIGTERM to a process, then SIGKILL if it doesn't exit.
func stopProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to %d: %w", pid, err)
	}

	// Wait briefly for graceful shutdown, then force kill.
	done := make(chan error, 1)
	go func() {
		_, err := proc.Wait()
		done <- err
	}()

	select {
	case <-done:
		return nil
	case <-time.After(3 * time.Second):
		proc.Signal(syscall.SIGKILL)
		return nil
	}
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pidStr := r.URL.Query().Get("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		http.Error(w, "Invalid PID", http.StatusBadRequest)
		return
	}

	// Don't allow stopping ourselves.
	if pid == os.Getpid() {
		http.Error(w, "Cannot stop the dashboard itself", http.StatusForbidden)
		return
	}

	if err := stopProcess(pid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Stopped process %d", pid)

	// Return updated table after a brief pause for the port to release.
	time.Sleep(500 * time.Millisecond)
	services, _ := getServices()
	tmpl := template.Must(template.ParseFiles("templates/partials/service_table.html"))
	tmpl.Execute(w, services)
}

func handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pidStr := r.URL.Query().Get("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		http.Error(w, "Invalid PID", http.StatusBadRequest)
		return
	}

	if pid == os.Getpid() {
		http.Error(w, "Cannot restart the dashboard itself", http.StatusForbidden)
		return
	}

	// Capture the command before killing the process.
	cmdLine, err := getProcessCommand(pid)
	if err != nil {
		http.Error(w, "Could not determine process command: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := stopProcess(pid); err != nil {
		http.Error(w, "Failed to stop: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Stopped process %d (command: %s), restarting...", pid, cmdLine)
	time.Sleep(500 * time.Millisecond)

	// Relaunch the process in the background.
	cmd := exec.Command("bash", "-c", cmdLine+" &")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to restart: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Detach so we don't wait for it.
	go cmd.Wait()

	log.Printf("Restarted: %s", cmdLine)

	// Wait for the new process to bind the port.
	time.Sleep(1 * time.Second)
	services, _ := getServices()
	tmpl := template.Must(template.ParseFiles("templates/partials/service_table.html"))
	tmpl.Execute(w, services)
}

func main() {
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/services", handleServices)
	http.HandleFunc("/stop", handleStop)
	http.HandleFunc("/restart", handleRestart)

	port := "8899"
	fmt.Printf("Homepage running at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
