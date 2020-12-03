package main

import (
	signalling "github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/signallingclient"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/transcontainer"
	log "github.com/sirupsen/logrus"
	"os"
)

func main() {

	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.DebugLevel)


	lifecycle := transcontainer.NewLifecycle(*signalling.NewSignallingClient("http://localhost:57778"))
	defer Defer()

	lifecycle.Start()

	// Block forever
	select {}
}

func Defer() {
	if err := recover(); err != nil {
		log.WithFields(
			log.Fields{
				"component": "main",
				"error": err,
			}).Error("Defer(): caught a panic!")

		panic(err)
	}
}

