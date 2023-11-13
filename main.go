package main

import (
	"bytes"
	"encoding/json"
	"flag"
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
	Type   string
	Status string
	Total  int
}

type DruidTasksExporter struct {
	Tasks *prometheus.Desc
}

func NewDruidTasksExporter() *DruidTasksExporter {
	return &DruidTasksExporter{
		Tasks: prometheus.NewDesc(
			"dte_druid_tasks_total",
			"Total number of Druid tasks per type and status.",
			[]string{"type", "status"},
			prometheus.Labels{},
		)}
}

func (d *DruidTasksExporter) RetrieveMetrics() []Task {

	query, _ := json.Marshal(map[string]string{
		"query": "SELECT type,status,count(*) AS total FROM sys.tasks GROUP BY status,type",
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
	for _, task := range tasks {
		ch <- prometheus.MustNewConstMetric(
			d.Tasks,
			prometheus.GaugeValue,
			float64(task.Total),
			task.Type,
			task.Status,
		)
	}
}
func ok(w http.ResponseWriter, _ *http.Request) {
	io.WriteString(w, "ok")
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