package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	addr     = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
	druidUri = flag.String("druid-uri", "http://BROKER:8082/druid/v2/sql/", "The URI to reach Druid's router or broker SQL API.")
)

type Task struct {
	Type         string
	RunnerStatus string
	Total        int
}

type DruidTasksExporter struct {
	Tasks *prometheus.Desc
}

func NewDruidTasksExporter() *DruidTasksExporter {
	return &DruidTasksExporter{
		Tasks: prometheus.NewDesc(
			"dte_druid_tasks_total",
			"Total number of Druid tasks per type and status.",
			[]string{"type", "runner_status"},
			prometheus.Labels{},
		)}
}

func (d *DruidTasksExporter) RetrieveMetrics() []Task {

	query, _ := json.Marshal(map[string]string{
		"query": "SELECT type,runner_status,count(*) AS total FROM sys.tasks GROUP BY type,runner_status",
	})

	reqBody := bytes.NewBuffer(query)
	resp, err := http.Post(*druidUri, "application/json", reqBody)
	if err != nil {
		log.Fatalf("An Error occured while making the request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("An Error occured while reading the response: %v", err)
	}

	var tasks []Task
	err = json.Unmarshal(body, &tasks)
	if err != nil {
		log.Fatalf("An Error occured while unmarshalling %s: %v", body, err)
	}
	return tasks
}

func (c *DruidTasksExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Tasks
}

func (d *DruidTasksExporter) Collect(ch chan<- prometheus.Metric) {
	tasks := d.RetrieveMetrics()
	runnerStatuses := []string{"NONE", "PENDING", "RUNNING", "WAITING"}
	taskTypes := []string{"single_phase_sub_task", "index", "index_parallel", "kill", "compact"}

	for _, status := range runnerStatuses {
		for _, taskType := range taskTypes {
			is_present := false
			for _, task := range tasks {
				if task.Type == taskType && task.RunnerStatus == status {
					is_present = true
				}
			}
			if !is_present {
				tasks = append(tasks, Task{Type: taskType, RunnerStatus: status, Total: 0})

			}

		}
	}
	for _, task := range tasks {
		ch <- prometheus.MustNewConstMetric(
			d.Tasks,
			prometheus.GaugeValue,
			float64(task.Total),
			task.Type,
			task.RunnerStatus,
		)
	}
}
func ok(w http.ResponseWriter, _ *http.Request) {
	_, err := io.WriteString(w, "ok")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		fmt.Println("Error writing response:", err)
		return
	}
}

func main() {
	flag.Parse()

	druid := NewDruidTasksExporter()
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(druid)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	http.HandleFunc("/", ok)
	log.Printf("The server is listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
