#### Performance Metrics Calculator
___

This tool calculates the following metrics 
- Number of requests sent
- Fastest Response Time in ms
- Slowest Response Time in ms
- Mean Response Time
- Median Response Time
- The percentage of requests that succeeded
- Any error codes returned that weren't a success
- The size in bytes of the smallest response
- The size in bytes of the largest response


###### Usage
___
1. Install Go
2. Make sure the GOROOT and GOPATH are set correctly
3. Install the following packages (not sure if Go automatically downloads the libraries)
    - "github.com/galdor/go-cmdline"
    - "github.com/montanaflynn/stats"
    - "github.com/sirupsen/logrus"
4. cd into the repo
5. Commands
    - go run . --help
    - go run . --url {url}
    - go run . --url {url} --profile {Number of requests}


###### Working
___
- Definition of Response Time for this tool: The time taken to establish the connection to the server and receive the _all_ the data sent by the server. Hence, if the data sends a lot of data, the response time will increase.
- If there are no successful responses from the server, the *Fastest Response Time*, *Slowest Response Time*, *Mean Response Time*, *Median Response Time*, *Size of the smallest response*, *Size of the largest response* are for the responses with status codes other than OK (200). 
- The tool automatically reads the protocol from the URL and forms a socket connection with ssl.
- Currently supports only _http_ and _https_.
- The connection request times out after 7s.

###### Screenshots
___
![helpImage](assets/help.PNG)

![workerPageFetch](assets/url-response.PNG)

![workerLinksPageFetch](assets/url-links-success.PNG)

![workerLinksPageFetch](assets/url-failure.PNG)


###### Comparison
___
| Number of Requests 	| Amazon 	                            | Google 	                            | Worker 	                            |
|--------------------	|--------	                            |--------	                            |--------	                            |
|        100          	|![Amazon100](assets/amazon-url-100.PNG)|![Google100](assets/google-url-100.PNG)|![worker100](assets/worker-url-100.PNG)|
|                    	|        	                            |        	                            |        	                            |
|                    	|        	                            |        	                            |        	                            |