package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/ziutek/rrd"
)

var logger *log.Logger

// Printer holds the host name
type Printer struct {
	Host string `json:"host"`
	Port uint16 `json:"port,omitempty"`
}

type Configuration struct {
	Printer  Printer `json:"printer"`
	CertFile string  `json:"ca-cert"`
	LogFile  string  `json:"logfile"`
	RRDFile  string  `json:"rrdfile"`
}

func main() {
	conf := loadConfiguration()
	setUpLogging(conf.LogFile)
	setUpRRD(conf.RRDFile)
	u := getHTML(false, conf)
	updateUsage(u, conf.RRDFile)
}

func loadConfiguration() *Configuration {
	conf, _ := os.Open("conf.json")
	defer conf.Close()
	decoder := json.NewDecoder(conf)
	configuration := &Configuration{}

	err := decoder.Decode(configuration)
	if err != nil {
		log.Fatal(err)
	}

	return configuration
}

func setUpLogging(logfile string) {
	// open log file
	file, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}

	logger = log.New(file, "hp-monitor: ", log.Lshortfile|log.LstdFlags)
	logger.Println("Starting hp-monitor")
}

func getHTML(test bool, conf *Configuration) *usage {
	if test {
		/* testing mock */
		file, _ := os.Open("usage.html")
		defer file.Close()
		return newUsage(file)
	} else {
		/* actual */
		p := x509.NewCertPool()
		cert, err := ioutil.ReadFile(conf.CertFile)
		if err != nil {
			log.Fatal(err)
		}
		p.AppendCertsFromPEM(cert)

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: p},
		}
		client := &http.Client{Transport: tr}
		res, err := client.Get(conf.Printer.URL())
		if err != nil {
			logger.Fatal(err)
		}
		return newUsage(res.Body)
	}

}

func (p *Printer) URL() string {
	baseURL := fmt.Sprintf("https://%s", p.Host)
	URL, _ := url.Parse(baseURL)

	if p.Port != 443 {
		URL.Path += fmt.Sprintf(":%d", p.Port)
	}
	URL.Path += "/hp/device/this.LCDispatcher"

	/* append query params*/
	v := url.Values{}
	v.Set("nav", "hp.Usage")
	URL.RawQuery = v.Encode()

	return URL.String()
}

func setUpRRD(dbfile string) {
	start := time.Date(2015, time.July, 1, 0, 0, 0, 0, time.UTC)
	c := rrd.NewCreator(dbfile, start, 3600)

	/* setup DSes */
	c.DS("letter-simplex", "GAUGE",
		7200, /* heartbeat */
		"U",  /* min */
		"U",  /* max */
	)
	c.DS("letter-duplex", "GAUGE",
		7200, /* heartbeat */
		"U",  /* min */
		"U",  /* max */
	)

	/* setup RRAs*/
	c.RRA("AVERAGE",
		0.5, /* xff */
		1,   /* steps */
		168, /* rows */
	)

	if err := c.Create(false); os.IsNotExist(err) {
		logger.Fatal(err)
	}
}

func updateUsage(u *usage, dbfile string) {
	delta := sinceLastUpdate(dbfile)
	if delta.Seconds() > 3300 {
		logger.Println("Updating RRD with usage...")
		up := rrd.NewUpdater(dbfile)
		up.SetTemplate("letter-simplex:letter-duplex")

		err := up.Update(time.Now(), u.simplex(), u.duplex())
		if err != nil {
			logger.Fatal(err)
		}
	} else {
		logger.Println("Too soon...")
	}
}

func sinceLastUpdate(dbfile string) time.Duration {
	ri, _ := rrd.Info(dbfile)
	lastUpdate := int64(ri["last_update"].(uint))
	t := time.Unix(lastUpdate, 0)
	delta := time.Since(t)

	return delta
}
