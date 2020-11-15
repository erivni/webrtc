package main

import (
	signalling "github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/signallingclient"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/transcontainer"
)

func main() {

	tc := transcontainer.NewLifecycle(*signalling.NewSignallingClient("http://34.250.45.79:57778"))

	tc.Start()

	// Block forever
	select {}
}
