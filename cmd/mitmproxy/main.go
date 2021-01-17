package main

import (
	"flag"
	"os"

	"github.com/lqqyt2423/go-mitmproxy/addon"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	addr      string
	dump      string // dump filename
	dumpLevel int    // dump level

	filterHost string
	filterUri  string
	filterBody string
	redisUri   string
}

func loadConfig() *Config {
	config := new(Config)

	flag.StringVar(&config.addr, "addr", ":9080", "proxy listen addr")
	flag.StringVar(&config.dump, "dump", "", "dump filename")
	flag.IntVar(&config.dumpLevel, "dump_level", 0, "dump level: 0 - header, 1 - header + body")

	flag.StringVar(&config.filterHost, "host", "", "regex str for host")
	flag.StringVar(&config.filterUri, "uri", "", "regex str for uri")
	flag.StringVar(&config.filterBody, "ctpye", "", "regex str for content_type")
	flag.StringVar(&config.redisUri, "redis", "", "")
	flag.Parse()

	return config
}

func main() {
	log.SetLevel(log.InfoLevel)
	log.SetReportCaller(false)
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	config := loadConfig()

	opts := &proxy.Options{
		Addr:              config.addr,
		StreamLargeBodies: 1024 * 1024 * 5,
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}

	if config.dump == "dump" {
		dumper := addon.NewDumper(config.dump, config.dumpLevel)
		p.AddAddon(dumper)
	}
	if config.dump == "filter" {
		dumper := addon.NewFilterDumper(config.filterHost, config.filterUri, config.filterBody, config.redisUri)
		p.AddAddon(dumper)
	}

	log.Fatal(p.Start())
}
