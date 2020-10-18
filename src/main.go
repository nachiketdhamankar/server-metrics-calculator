package main

import (
	"crypto/tls"
	"fmt"
	mathPackage "github.com/montanaflynn/stats"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"time"
)

type url struct {
	hostname string
	path     string
	protocol string
}
type minMaxResponseTimes struct {
	minResponseTime int64
	maxResponseTime int64
}
type profilingStats struct {
	statusCode int
	response   []byte
	duration   time.Duration
}
type processedStats struct {
	fastestTime       int64
	slowestTime       int64
	meanTime          float64
	medianTime        float64
	requestsSucceeded float32
	errorStatusCodes  []int
	smallestResponse  int
	largestResponse   int
}

func getHttpsResponse(addr url) profilingStats {
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

func getHttpResponse(addr url) profilingStats {
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

func makeRequest(addr url) profilingStats {
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

func getMeanTimesInms(stats []profilingStats) processedStats {
	if len(stats) < 1 {
		return processedStats{}
	}

	var finalStats processedStats
	var successResponseTime []int64
	var failureResponseTime []int64
	var successEdgeTimes minMaxResponseTimes
	var failureEdgeTimes minMaxResponseTimes

	// Set default values for min and max values
	successEdgeTimes.maxResponseTime = -1 << 63
	failureEdgeTimes.maxResponseTime = -1 << 63
	successEdgeTimes.minResponseTime = 1<<63 - 1
	failureEdgeTimes.minResponseTime = 1<<63 - 1

	// Initialise the smallest and largest response to 1st element of stats
	finalStats.largestResponse = len(stats[0].response)
	finalStats.smallestResponse = len(stats[0].response)

	// Calculate min, max for success times and failure times
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

			// Find min, max for Success Response Times
			if currentDuration < successEdgeTimes.minResponseTime {
				successEdgeTimes.minResponseTime = currentDuration
			} else if currentDuration > successEdgeTimes.maxResponseTime {
				successEdgeTimes.maxResponseTime = currentDuration
			}

			// Find largest and smallest byte size for success
			if len(stat.response) < finalStats.smallestResponse {
				finalStats.smallestResponse = len(stat.response)
			} else if len(stat.response) > finalStats.largestResponse {
				finalStats.largestResponse = len(stat.response)
			}

		} else {

			currentDuration := stat.duration.Milliseconds()
			failureResponseTime = append(failureResponseTime, currentDuration)

			// Find min, max for Failed Response Times
			if currentDuration < failureEdgeTimes.minResponseTime {
				failureEdgeTimes.minResponseTime = currentDuration
			} else if currentDuration > failureEdgeTimes.maxResponseTime {
				failureEdgeTimes.maxResponseTime = currentDuration
			}

			// Add failure codes to the final stats struct
			finalStats.errorStatusCodes = append(finalStats.errorStatusCodes, stat.statusCode)

		}
	}

	log.WithFields(log.Fields{
		"successEdgeTimes": successEdgeTimes,
		"failureEdgeTimes": failureEdgeTimes,
	}).Info("Edge Times")


	finalStats.fastestTime = successEdgeTimes.minResponseTime
	finalStats.slowestTime = successEdgeTimes.maxResponseTime

	log.WithFields(log.Fields{
		"successTimes": successResponseTime,
		"failureTimes": failureResponseTime,
	}).Debug("Response Times")

	// Calculate mean
	successMeanTime := getMean(successResponseTime)

	var failureMeanTime float64 = 0
	if len(failureResponseTime) > 0 {
		failureMeanTime = getMean(failureResponseTime)
	}
	log.WithFields(log.Fields{
		"successMeanTime": successMeanTime,
		"failureMeanTime": failureMeanTime,
	}).Info("Mean Times")
	finalStats.meanTime = successMeanTime

	// Calculate median
	finalStats.medianTime, _ = mathPackage.Median(mathPackage.LoadRawData(successResponseTime))
	return finalStats
}

func processStats(stats []profilingStats) {
	meanTimeInms := getMeanTimesInms(stats)
	fmt.Print(meanTimeInms)
}
func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetReportCaller(true)
	//log.SetLevel(log.InfoLevel)
	log.SetLevel(log.ErrorLevel)
}

func main() {

	httpAddr := url{
		hostname: "google.in",
		path:     "/",
		protocol: "https",
	}

	var totalStats []profilingStats
	for i := 1; i <= 5; i++ {
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
	processStats(totalStats)

}
