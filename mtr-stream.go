package main

import (
	"./mqttc"
	"encoding/json"
	"fmt"
	"git.eclipse.org/gitroot/paho/org.eclipse.paho.mqtt.golang.git"
	log "github.com/Sirupsen/logrus"
	"github.com/mgutz/ansi"
	"gopkg.in/alecthomas/kingpin.v1"
	"os"
	"strings"
	"time"
)

var (
	countryCode = ""
)

type Host struct {
	IP          string  `json:"ip"`
	Name        string  `json:"hostname"`
	Hop         int     `json:"hop-number"`
	Sent        int     `json:"sent"`
	LostPercent float64 `json:"lost-percent"`
	Last        float64 `json:"last"`
	Avg         float64 `json:"avg"`
	Best        float64 `json:"best"`
	Worst       float64 `json:"worst"`
	StDev       float64 `json:"standard-dev"`
}

type Report struct {
	Time        time.Time       `json:"time"`
	Hosts       []*Host         `json:"hosts"`
	Hops        int             `json:"hops"`
	ElapsedTime time.Duration   `json:"elapsed_time"`
	Location    *ReportLocation `json:"location"`
}

// slightly simpler struct than the one provided by geoipc
type ReportLocation struct {
	IP          string  `json:"ip"`
	CountryCode string  `json:"country_code"`
	CountryName string  `json:"country_name"`
	City        string  `json:"city"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

func printReport(client *mqtt.MqttClient, msg mqtt.Message) {
	var report Report
	err := json.Unmarshal(msg.Payload(), &report)
	if err != nil {
		log.Error("Error decoding json report")
	}

	cc := report.Location.CountryCode
	if cc == "" {
		log.Warn("Discarding report with no country code")
		return
	}

	// Skip countries not matching the filter
	if countryCode != "" && cc != countryCode {
		return
	}

	fmt.Println(ansi.Color(report.Location.CountryName, "white+b") + " - " + report.Location.IP)
	fmt.Println("------------------------")
	fmt.Printf("%-4s %-18s %-6s %-8s %-8s %-8s %-8s %-8s %-8s\n",
		"Hop", "IP", "Sent", "Loss %", "Last", "Avg", "Best", "Worst", "StdDev")
	for _, host := range report.Hosts {
		fmt.Printf("%-4d %-18s %-6d %-8.1f %-8.1f %-8.1f %-8.1f %-8.1f %-8.1f\n",
			host.Hop, host.IP, host.Sent, host.LostPercent, host.Last, host.Avg, host.Best, host.Worst, host.StDev)
	}
	println()
}

func parseBrokerUrls(brokerUrls string) []string {
	tokens := strings.Split(brokerUrls, ",")
	for i, url := range tokens {
		tokens[i] = strings.TrimSpace(url)
	}

	return tokens
}

func main() {
	kingpin.Version(PKG_VERSION)

	brokerUrls := kingpin.Flag("broker-urls", "Comman separated MQTT broker URLs").
		Required().Default("").OverrideDefaultFromEnvar("MQTT_URLS").String()

	cafile := kingpin.Flag("cafile", "CA certificate when using TLS (optional)").
		String()

	topic := kingpin.Flag("topic", "MQTT topic").
		Default("/metrics/mtr").String()

	clientID := kingpin.Flag("clientid", "Use a custom MQTT client ID").String()

	insecure := kingpin.Flag("insecure", "Don't verify the server's certificate chain and host name.").
		Default("false").Bool()

	debug := kingpin.Flag("debug", "Print debugging messages").
		Default("false").Bool()

	fCountryCode := kingpin.Flag("country-code", "Filter reports by country code").String()

	kingpin.Parse()

	countryCode = *fCountryCode

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	var err error

	if *cafile != "" {
		if _, err := os.Stat(*cafile); err != nil {
			log.Fatalf("Error reading CA certificate %s", err.Error())
			os.Exit(1)
		}
	}

	urlList := parseBrokerUrls(*brokerUrls)

	if *clientID == "" {
		*clientID, err = os.Hostname()
		if err != nil {
			log.Fatal("Can't get the hostname to use it as the ClientID, use --clientid option")
		}
	}
	log.Debugf("MQTT Client ID: %s", *clientID)

	for _, urlStr := range urlList {
		args := mqttc.Args{
			BrokerURLs:    []string{urlStr},
			ClientID:      *clientID,
			Topic:         *topic,
			TLSCACertPath: *cafile,
			TLSSkipVerify: *insecure,
		}

		log.Debug("Starting mqttc client")
		c := mqttc.Subscribe(printReport, &args)
		defer c.Disconnect(0)
	}

	// wait endlessly
	var loop chan bool
	loop <- true
}
