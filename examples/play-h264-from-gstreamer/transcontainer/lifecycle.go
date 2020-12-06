package transcontainer

import (
	"fmt"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/gst"
	signalling "github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/signallingclient"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/webrtc"
	log "github.com/sirupsen/logrus"
)

type Lifecycle struct {

	uiConnection *webrtc.WebRTCClient
	clientConnection *webrtc.WebRTCServer

	abrPlayer      *gst.Pipeline
	transcontainer *Transcontainer
}

func NewLifecycle(signallingClient signalling.SignallingClient) *Lifecycle {
	lifecycle := &Lifecycle{}
	lifecycle.uiConnection = webrtc.NewWebRTCClient(signallingClient, nil, lifecycle.OnUiStateChange, nil )
	lifecycle.clientConnection = webrtc.NewWebRTCServer(signallingClient, nil, lifecycle.OnClientStateChange, nil)
	lifecycle.transcontainer = NewTranscontainer(lifecycle.uiConnection, lifecycle.clientConnection, lifecycle.abrPlayer, nil, nil)
	return lifecycle
}

func (lifecycle *Lifecycle) Start() {

	log.WithFields(
		log.Fields{
			"component": 		"lifecycle",
			"state": 			"start",
		}).Debug("starting lifecycle")

	pipelineStr := fmt.Sprintf("souphttpsrc location=http://hyperscale.coldsnow.net:8080/bbb_360_abr.m3u8 ! hlsdemux ! decodebin3 name=demux caps=video/x-h264,stream-format=byte-stream ! appsink name=video demux. ! queue ! audioconvert ! audioresample ! opusenc ! appsink name=audio")
	log.WithFields(
		log.Fields{
			"component": "lifecycle",
			"state"	: 	 "start",
			"abrPlayer": pipelineStr,
		}).Debug("setting abrPlayer.")

	//lifecycle.abrPlayer = gst.CreatePipeline(pipelineStr)
	lifecycle.abrPlayer = gst.CreatePipeline(pipelineStr, nil, nil, "abr", lifecycle.transcontainer.processSample)
	lifecycle.transcontainer.abrPlayer = lifecycle.abrPlayer

	log.WithFields(
		log.Fields{
			"component": 		"lifecycle",
			"state": 			"start",
		}).Debug("starting client connection")
	lifecycle.clientConnection.Connect()
}

func (lifecycle *Lifecycle) Stop() {

	log.WithFields(
		log.Fields{
			"component": 		"lifecycle",
			"state": 			"start",
		}).Debug("stopping lifecycle. closing all connections")
	lifecycle.clientConnection.Disconnect()
	lifecycle.uiConnection.Disconnect()
	lifecycle.transcontainer.Stop()
}

func (lifecycle *Lifecycle) OnClientStateChange(state webrtc.State) {
	log.WithFields(
		log.Fields{
			"component": 		"lifecycle",
			"state":			"onclientstatechanged",
			"clientState":		state,
		}).Debug("client connection changed")

	switch state {
	case webrtc.INITIATE:
		log.WithFields(
			log.Fields{
				"component": 		"lifecycle",
				"state":			"onclientstatechanged",
				"clientState":		state,
			}).Info("client has a waiting offer, staring ui connection")
		lifecycle.uiConnection.Connect(lifecycle.clientConnection.ConnectionId)
		break
	case webrtc.CONNECTED:
		if lifecycle.uiConnection.State == webrtc.CONNECTED && lifecycle.transcontainer.State != STARTED {
			log.WithFields(
				log.Fields{
					"component": 		"lifecycle",
					"state":			"onclientstatechanged",
					"clientState":		state,
				}).Info("client and ui connections are ready, starting transcontainer and abrPlayer")
			lifecycle.transcontainer.Start()
		}
		break
	case webrtc.FAILED, webrtc.IDLE:
		lifecycle.Stop()
		break
	case webrtc.DISCONNECTED:
		break // ignored
	default:
		break
	}
}

func (lifecycle *Lifecycle) OnUiStateChange(state webrtc.State) {

	log.WithFields(
		log.Fields{
			"component": 		"lifecycle",
			"state":			"onuitstatechanged",
			"uiState":		state,
		}).Debug("ui connection changed")

	switch state {
	case webrtc.CONNECTED:
		if lifecycle.clientConnection.State == webrtc.CONNECTED && lifecycle.transcontainer.State != STARTED {
			log.WithFields(
				log.Fields{
					"component": 		"lifecycle",
					"state":			"onuitstatechanged",
					"uiState":		state,
				}).Info("client and ui connections are ready, starting transcontainer and abrPlayer")
			lifecycle.transcontainer.Start()
		}
		break
	case webrtc.FAILED, webrtc.IDLE:
		lifecycle.Stop()
		break
	case webrtc.INITIATE, webrtc.DISCONNECTED:
		break // ignored
	default:
		break
	}
}
