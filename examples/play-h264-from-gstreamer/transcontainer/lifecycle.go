package transcontainer

import (
	signalling "github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/signallingclient"
	log "github.com/sirupsen/logrus"
)

type Lifecycle struct {
	state State
	transcontainer *Transcontainer
}

func NewLifecycle(signallingClient signalling.SignallingClient) *Lifecycle {
	lifecycle := &Lifecycle{state: STOP}
	lifecycle.transcontainer = NewTranscontainer(signallingClient, lifecycle.onTranscontainerStateChanged, nil)
	return lifecycle
}

func (lifecycle *Lifecycle) Start() {

	if lifecycle.state == START {
		return
	}

	lifecycle.state = START
	log.WithFields(
		log.Fields{
			"component": 		"lifecycle",
			"state": 			lifecycle.state,
		}).Info("starting lifecycle..")

	lifecycle.transcontainer.Start()
}

func (lifecycle *Lifecycle) Stop() {

	if lifecycle.state == STOP {
		return
	}

	lifecycle.state = STOP
	log.WithFields(
		log.Fields{
			"component": 		"lifecycle",
			"state": 			lifecycle.state,
		}).Info("stopping lifecycle..")
	lifecycle.transcontainer.Stop()
}

func (lifecycle *Lifecycle) Restart() {
	lifecycle.Stop()
	lifecycle.Start()
}

func (lifecycle *Lifecycle) onTranscontainerStateChanged(state State) {

	switch state {
	case START:
		break
	case STOP:
		log.WithFields(
			log.Fields{
				"component": 			"lifecycle",
				"state":				 state,
			}).Info("transcontainer stopped, restarting lifecycle..")
		lifecycle.Restart()
		break
	case STREAM:
		break
	default:
		break
	}
}
