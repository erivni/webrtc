package main

import (
	signalling "github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/signallingclient"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/transcontainer"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
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


	tc := transcontainer.NewLifecycle(*signalling.NewSignallingClient("http://34.250.45.79:57778"))
	defer Defer(tc)

	tc.Start()

	// Block forever
	select {}
}

func Defer(tc *transcontainer.Lifecycle) {
	if err := recover(); err != nil {
		log.WithFields(
			log.Fields{
				"lifecycleState": tc.State,
				"connectionId": tc.ConnectionId,
				"error": err,
			}).Error("Defer(): caught a panic!")

		time.Sleep(5 * time.Second)

		defer Defer(tc)
		tc.Restart()
	}
}

