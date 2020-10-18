package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/galdor/go-cmdline"
	mathPackage "github.com/montanaflynn/stats"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type customUrl struct {
	hostname string
	path     string
	protocol string
}
type profilingStats struct {
	statusCode int
	response   []byte
	duration   time.Duration
}
type processedStats struct {
	fastestTime       float64
	slowestTime       float64
	meanTime          float64
	medianTime        float64
	requestsSucceeded float32
	errorStatusCodes  []int
	smallestResponse  int
	largestResponse   int
}

func getHttpsResponse(addr customUrl) profilingStats {
	timeout, _ := time.ParseDuration("7s")
	dialer := net.Dialer{
		Timeout: timeout,
	}

	start := time.Now()

	conn, err := tls.DialWithDialer(&dialer, "tcp", addr.hostname+":"+addr.protocol, nil)
	if err != nil {
		fmt.Println("Error in recv")
		fmt.Println(err)
	}

	_, err = fmt.Fprintf(conn, "GET "+addr.path+" HTTP/1.0\r\nHost: "+addr.hostname+"\r\n\r\n")
	if err != nil {
		fmt.Println(err)
	}
	resp, err := ioutil.ReadAll(conn)
	if err != nil {
		fmt.Printf("Error while reading from connection: %s\n", err)
	}

	duration := time.Since(start)

	response := string(resp)
	statusCode, err := strconv.Atoi(response[9:12])
	if err != nil {
		fmt.Printf("Error while getting statusCode: %s\n", err)
	}
	err = conn.Close()
	if err != nil {
		fmt.Printf("Error while closing connection: %s\n", err)
	}
	return profilingStats{
		statusCode: statusCode,
		response:   resp,
		duration:   duration,
	}
}

func getHttpResponse(addr customUrl) profilingStats {
	log.WithFields(log.Fields{
		"urlHostName": addr.hostname,
		"urlPath":     addr.path,
		"urlProtocol": addr.protocol,
	}).Debug("Sending requests now")

	start := time.Now()

	conn, err := net.Dial("tcp", addr.hostname+":"+addr.protocol)
	if err != nil {
		fmt.Printf("Error while creating connection: %s\n", err)
	}
	test1 := "GET " + addr.path + " HTTP/1.0\r\nHost: " + addr.hostname + "\r\n\r\n"

	_, err = fmt.Fprintf(conn, test1)
	if err != nil {
		fmt.Printf("Error while sending request: %s\n", err)
	}

	resp, err := ioutil.ReadAll(conn)
	if err != nil {
		fmt.Printf("Error while reading from connection: %s\n", err)
	}

	duration := time.Since(start)

	response := string(resp)
	statusCode, err := strconv.Atoi(response[9:12])
	if err != nil {
		fmt.Printf("Error while getting statusCode: %s\n", err)
	}

	err = conn.Close()
	if err != nil {
		fmt.Printf("Error while closing connection: %s\n", err)
	}

	return profilingStats{
		statusCode: statusCode,
		response:   resp,
		duration:   duration,
	}
}

func makeRequest(addr customUrl) profilingStats {
	switch addr.protocol {
	case "http":
		return getHttpResponse(addr)
	case "https":
		return getHttpsResponse(addr)
	default:
		log.Error("Not supported format " + addr.protocol)
		log.Exit(1)
	}
	return profilingStats{}
}

func getMean(values []int64) float64 {
	if len(values) < 1 {
		log.Debug("Attempted to find a mean for array with no values")
		return 0.0
	}
	var sum float64 = 0.0
	for _, value := range values {
		sum += float64(value)
	}
	return sum / float64(len(values))
}

func processStats(stats []profilingStats) processedStats {
	if len(stats) < 1 {
		return processedStats{}
	}
	var finalStats processedStats
	var successResponseTime []int64
	var failureResponseTime []int64

	// Initialise the smallest and largest response to 1st element of stats
	finalStats.largestResponse = len(stats[0].response)
	finalStats.smallestResponse = len(stats[0].response)

	// Append the success times and the failure times into appropriate arrays
	for _, stat := range stats {
		log.WithFields(log.Fields{
			"statResponseTime":    stat.duration,
			"statResponseCode":    stat.statusCode,
			"statResponseByteLen": len(stat.response),
		}).Info("Processing stat: ")

		if stat.statusCode == 200 {
			currentDuration := stat.duration.Milliseconds()
			successResponseTime = append(successResponseTime, currentDuration)

			// Find largest and smallest byte size for success
			if len(stat.response) < finalStats.smallestResponse {
				finalStats.smallestResponse = len(stat.response)
			} else if len(stat.response) > finalStats.largestResponse {
				finalStats.largestResponse = len(stat.response)
			}
		} else {
			currentDuration := stat.duration.Milliseconds()
			failureResponseTime = append(failureResponseTime, currentDuration)

			// Add failure codes to the final stats struct
			finalStats.errorStatusCodes = append(finalStats.errorStatusCodes, stat.statusCode)
		}
	}

	responseTimes := append(failureResponseTime, successResponseTime...)
	finalStats.fastestTime, _ = mathPackage.Min(mathPackage.LoadRawData(responseTimes))
	finalStats.slowestTime, _ = mathPackage.Max(mathPackage.LoadRawData(responseTimes))

	log.WithFields(log.Fields{
		"successTimes": successResponseTime,
		"failureTimes": failureResponseTime,
	}).Debug("Response Times")

	// Calculate mean
	var meanTime float64 = 0.0
	if len(successResponseTime) > 0 {
		meanTime = getMean(successResponseTime)
	} else if len(failureResponseTime) > 0 {
		meanTime = getMean(failureResponseTime)
	}
	log.WithFields(log.Fields{
		"meanTime": meanTime,
	}).Debug("Mean Time")
	finalStats.meanTime = meanTime

	// Calculate median
	var medianTime float64 = 0.0
	if len(successResponseTime) > 0 {
		medianTime, _ = mathPackage.Median(mathPackage.LoadRawData(successResponseTime))
	} else {
		medianTime, _ = mathPackage.Median(mathPackage.LoadRawData(failureResponseTime))
	}
	log.WithFields(log.Fields{
		"medianTime": finalStats.medianTime,
	}).Debug("Median Time")
	finalStats.medianTime = medianTime

	// Calculate percentage of requests that succeeded
	if len(successResponseTime)+len(failureResponseTime) <= 0 {
		finalStats.requestsSucceeded = 0
	} else {
		finalStats.requestsSucceeded = float32(len(successResponseTime) / (len(failureResponseTime) + len(successResponseTime)) * 100)
	}
	return finalStats
}

func fetchPage(httpAddr customUrl) string {
	response := makeRequest(httpAddr)
	return string(response.response)
}
func formatProcessedOutput(stats processedStats, profileCount int) string {
	var str strings.Builder
	str.WriteString(fmt.Sprintf("Number of requests: %d\n", profileCount))
	str.WriteString(fmt.Sprintf("Fastest Time: %g ms\n", stats.fastestTime))
	str.WriteString(fmt.Sprintf("Slowest Time: %g ms\n", stats.slowestTime))
	str.WriteString(fmt.Sprintf("Mean Time: %g ms\n", stats.meanTime))
	str.WriteString(fmt.Sprintf("Median Time: %g ms\n", stats.medianTime))
	str.WriteString(fmt.Sprintf("Percentage requests that succeeded: %g%% \n", stats.requestsSucceeded))
	if len(stats.errorStatusCodes) > 0 {
		errorCodesStr, _ := json.Marshal(stats.errorStatusCodes)
		str.WriteString(fmt.Sprintf("Error codes: [%s]\n", strings.Trim(string(errorCodesStr), "[]")))
	} else {
		str.WriteString(fmt.Sprintf("Error codes: []\n"))
	}
	str.WriteString(fmt.Sprintf("Size of bytes of the largest response: %d bytes\n", stats.largestResponse))
	str.WriteString(fmt.Sprintf("Size of bytes of the smalleset response: %d bytes\n", stats.smallestResponse))
	return str.String()

}
func fetchProfilingMetrics(httpAddr customUrl, profileCount int) string {
	var totalStats []profilingStats
	for i := 1; i <= profileCount; i++ {
		req := makeRequest(httpAddr)
		totalStats = append(totalStats, req)
		log.WithFields(log.Fields{
			"responseStatusCode":   req.statusCode,
			"responseDurationInms": req.duration.Milliseconds(),
			"response":             string(req.response),
		}).Debug("Profiling stats")

		log.WithFields(log.Fields{
			"responseStatusCode":   req.statusCode,
			"responseDurationInms": req.duration.Milliseconds(),
		}).Info("Profiling stats")
	}
	return formatProcessedOutput(processStats(totalStats), profileCount)
}

func init() {
	// Set logging
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetReportCaller(true)
	//Set default logging level
	log.SetLevel(log.ErrorLevel)
}

func isValidUrl(toTest string) (customUrl, bool) {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return customUrl{}, false
	}
	u, err := url.Parse(toTest)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return customUrl{}, false
	}

	if u.Path == "" {
		return customUrl{
			hostname: u.Host,
			path:     "/",
			protocol: u.Scheme,
		}, true
	} else {
		return customUrl{
			hostname: u.Host,
			path:     u.Path,
			protocol: u.Scheme,
		}, true
	}

}

func main() {
	cmdParser := cmdline.New()
	cmdParser.AddOption("u", "url", "value", "Enter the url to process")
	cmdParser.AddOption("p", "profile", "value", "Enter the number of requests to be sent")
	cmdParser.AddOption("d", "debug", "value", "Enter the debug level")
	cmdParser.Parse(os.Args)

	if cmdParser.IsOptionSet("debug") {
		debugLevel, err := strconv.Atoi(cmdParser.OptionValue("debug"))
		if err != nil || debugLevel <= 0 || debugLevel > 5 {
			log.Error("Enter valid positive number for debugLevel. Between 1 - 5")
			log.Exit(1)
		}
		log.SetLevel(log.Level(debugLevel))
	}

	if cmdParser.IsOptionSet("url") == false {
		log.Error("Missing url. Refer --help")
		log.Exit(1)
	}

	inputUrl := cmdParser.OptionValue("url")
	httpAddr, isValidUrl := isValidUrl(inputUrl)
	if isValidUrl == false {
		log.Error("Enter valid url")
		log.Exit(1)
	}

	if cmdParser.IsOptionSet("profile") {
		profileCount, err := strconv.Atoi(cmdParser.OptionValue("profile"))
		if err != nil || profileCount <= 0 {
			log.Error("Enter valid positive number for profile. Refer --help")
			log.Exit(1)
		}

		log.WithFields(log.Fields{
			"URL Hostname": httpAddr.hostname,
			"URL Scheme":   httpAddr.protocol,
			"URL Path":     httpAddr.path,
			"profileCount": profileCount,
		}).Info("Fetch url metrics")

		fmt.Println(fetchProfilingMetrics(httpAddr, profileCount))
	} else {

		log.WithFields(log.Fields{
			"URL Hostname": httpAddr.hostname,
			"URL Scheme":   httpAddr.protocol,
			"URL Path":     httpAddr.path,
		}).Info("Fetch page")

		fmt.Println(fetchPage(httpAddr))
	}
}
