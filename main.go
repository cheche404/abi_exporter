package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type APIResponse struct {
	Status  int         `json:"status"`
	Message interface{} `json:"message"`
	Data    struct {
		AuthorizerDate string `json:"AuthorizerDate"`
		DateLimit      string `json:"DateLimit"`
	} `json:"data"`
	Error interface{} `json:"error"`
}

type URLConfig struct {
	URL              string `json:"url"`
	Label            string `json:"label"`
	OriginPrometheus string `json:"origin_prometheus"`
}

type Config struct {
	URLs []URLConfig `json:"urls"`
}

var (
	metric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dap_abi_cert_expired_day",
			Help: "Difference in days between DateLimit and current date",
		},
		[]string{"url", "origin_prometheus"}, // Adding origin_prometheus as a label
	)
	registry = prometheus.NewRegistry()
)

func init() {
	// Read configuration from file
	configFile := flag.String("config", "config.json", "Path to the configuration file")
	flag.Parse()

	// Register the metric
	registry.MustRegister(metric)

	// Initialize metrics with default values
	config, err := readConfig(*configFile)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	for _, urlConfig := range config.URLs {
		// Initialize metric values for each URL
		metric.WithLabelValues(urlConfig.URL, urlConfig.OriginPrometheus).Set(-1) // Set default value to -1
	}
}

func readConfig(filePath string) (Config, error) {
	var config Config
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(data, &config)
	return config, err
}

func fetchData(urlConfig URLConfig) {
	// Create a POST request
	req, err := http.NewRequest("POST", urlConfig.URL, nil)
	if err != nil {
		log.Printf("Error creating POST request for %s: %v", urlConfig.URL, err)
		metric.WithLabelValues(urlConfig.URL, urlConfig.OriginPrometheus).Set(-1) // Set default value on failure
		return
	}

	// Perform the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error performing POST request to %s: %v", urlConfig.URL, err)
		metric.WithLabelValues(urlConfig.URL, urlConfig.OriginPrometheus).Set(-1) // Set default value on failure
		return
	}
	defer resp.Body.Close()

	var apiResponse APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		log.Printf("Error decoding JSON response from %s: %v", urlConfig.URL, err)
		metric.WithLabelValues(urlConfig.URL, urlConfig.OriginPrometheus).Set(-1) // Set default value on failure
		return
	}

	// Check if DateLimit field is empty
	if apiResponse.Data.DateLimit == "" {
		log.Printf("DateLimit is empty for URL %s", urlConfig.URL)
		metric.WithLabelValues(urlConfig.URL, urlConfig.OriginPrometheus).Set(-1) // Set default value on failure
		return
	}

	// Parse DateLimit field
	dateLimit, err := time.Parse("2006-01-02 15:04:05", apiResponse.Data.DateLimit)
	if err != nil {
		log.Printf("Error parsing DateLimit from %s: %v", urlConfig.URL, err)
		metric.WithLabelValues(urlConfig.URL, urlConfig.OriginPrometheus).Set(-1) // Set default value on failure
		return
	}

	currentTime := time.Now()
	dateDiff := dateLimit.Sub(currentTime).Hours() / 24
	metric.WithLabelValues(urlConfig.URL, urlConfig.OriginPrometheus).Set(round(dateDiff, 2))
}

// round function to round the value to the specified number of decimal places
func round(value float64, precision int) float64 {
	scale := math.Pow(10, float64(precision))
	return math.Round(value*scale) / scale
}

func main() {
	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	// Fetch data periodically
	go func() {
		for {
			// Read configuration from file and update metrics
			configFile := "config.json"
			config, err := readConfig(configFile)
			if err != nil {
				log.Fatalf("Error reading config file: %v", err)
			}

			for _, urlConfig := range config.URLs {
				fetchData(urlConfig)
			}
			time.Sleep(10 * time.Hour)
		}
	}()

	log.Println("Starting server on :18000")
	log.Fatal(http.ListenAndServe(":18000", r))
}
