package transcontainer

import (
	"fmt"
	"github.com/pion/randutil"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/gst"
	signalling "github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/signallingclient"
	"github.com/pion/webrtc/v3/pkg/rtpbuffer"
	log "github.com/sirupsen/logrus"
	"os"
)

type State uint8

const (
	START 			State = 0
	SETUP 			State = 1
	CONNECT 		State = 2
	ESTABLISHED 	State = 3
	STREAMING 		State = 4
	FAILED 			State = 5
	STOP			State = 6
)

func (s State) String() string {
	switch s {
	case START:
		return "START"
	case SETUP:
		return "SETUP"
	case CONNECT:
		return "CONNECT"
	case ESTABLISHED:
		return "ESTABLISHED"
	case STREAMING:
		return "STREAMING"
	case FAILED:
		return "FAILED"
	case STOP:
		return "STOP"
	default:
		return fmt.Sprintf("%d", int(s))
	}
}

type Lifecycle struct {
	State           State
	ConnectionId    string
	PeerConnection  *webrtc.PeerConnection
	MediaEngine     webrtc.MediaEngine
	AudioTrack      *webrtc.Track
	VideoTrack      *webrtc.Track
	SignalingClient signalling.SignallingClient
	AbrPipeline     *gst.Pipeline
	UiPipeLine      *gst.Pipeline
	VideoJitter     *rtpbuffer.Jitter
}

func NewLifecycle(signalingClient signalling.SignallingClient) *Lifecycle {
	return &Lifecycle{SignalingClient: signalingClient}
}

func (tc *Lifecycle) Start(){

	// set state
	tc.State = START

	// get an available offer
	var err error
	tc.ConnectionId, err = tc.SignalingClient.GetQueue()
	if err != nil {
		panic(err)
	}

	offer, err := tc.SignalingClient.GetOffer(tc.ConnectionId)
	if err != nil {
		panic(err)
	}

	log.WithFields(
		log.Fields{
			"component": "lifecycle",
			"lifecycleState": tc.State,
			"connectionId": tc.ConnectionId,
		}).Info("got a offer, trying to connect to peer.")

	tc.Setup(*offer)
}

func (tc *Lifecycle) Setup(offer webrtc.SessionDescription){

	// set state
	tc.State = SETUP

	tc.MediaEngine = webrtc.MediaEngine{}
	err := tc.MediaEngine.PopulateFromSDP(offer)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "lifecycle",
				"lifecycleState": tc.State,
				"connectionId": tc.ConnectionId,
				"error": err.Error(),
			}).Warn("failed to get a valid offer..")
		panic(err)
	}

	settingEngine := webrtc.SettingEngine{}
	err = settingEngine.SetEphemeralUDPPortRange(50000, 50002)
	if err != nil {
		panic(err)
	}

	// Create a new RTCPeerConnection
	var api = webrtc.NewAPI(
		webrtc.WithMediaEngine(tc.MediaEngine),
		webrtc.WithSettingEngine(settingEngine),
	)
	tc.PeerConnection, err = api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}


	tc.VideoTrack, err = tc.PeerConnection.NewTrack(getPayloadType(tc.MediaEngine, webrtc.RTPCodecTypeVideo, "H264"), randutil.NewMathRandomGenerator().Uint32(), "video", "transcontainer")
	if err != nil {
		panic(err)
	}
	if _, err = tc.PeerConnection.AddTrack(tc.VideoTrack); err != nil {
		panic(err)
	}

	tc.AudioTrack, err = tc.PeerConnection.NewTrack(getPayloadType(tc.MediaEngine, webrtc.RTPCodecTypeAudio, "opus"), randutil.NewMathRandomGenerator().Uint32(), "audio", "transcontainer")
	if err != nil {
		panic(err)
	}
	if _, err = tc.PeerConnection.AddTrack(tc.AudioTrack); err != nil {
		panic(err)
	}

	tc.VideoJitter = rtpbuffer.NewJitter(tc.PeerConnection, tc.VideoTrack)
	gst.SetJitter(tc.VideoJitter)

	// events registration
	tc.PeerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.WithFields(
			log.Fields{
				"component": "lifecycle",
				"lifecycleState": tc.State,
				"connectionId": tc.ConnectionId,
				"connectionState": connectionState.String(),
			}).Info("connection state has changed.")

		if connectionState == webrtc.ICEConnectionStateConnected {
			tc.Stream()
			//iceConnectedCtxCancel()
		} else if connectionState == webrtc.ICEConnectionStateFailed ||
			connectionState == webrtc.ICEConnectionStateDisconnected {
			// set state
			tc.State = FAILED
			tc.Restart()
		}
	})

	tc.PeerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			log.WithFields(
				log.Fields{
					"component": "lifecycle",
					"lifecycleState": tc.State,
					"connectionId": tc.ConnectionId,
				}).Info("no more ice candidates, posting answer to signaling server.")

			err = tc.SignalingClient.SendAnswer(tc.ConnectionId, *tc.PeerConnection.LocalDescription())
			if err != nil {
				panic(err)
			}
		}
	})

	tc.PeerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		log.WithFields(
			log.Fields{
				"component": "lifecycle",
				"lifecycleState": tc.State,
				"connectionId": tc.ConnectionId,
				"dataChannelId": d.ID(),
				"dataChannelLabel": d.Label(),
			}).Info("new data channel.")

		d.OnOpen(func() {
			log.WithFields(
				log.Fields{
					"component": "lifecycle",
					"lifecycleState": tc.State,
					"connectionId": tc.ConnectionId,
					"dataChannelId": d.ID(),
					"dataChannelLabel": d.Label(),
				}).Info("data channel is open.")

			hostname, _ := os.Hostname()
			d.SendText(fmt.Sprintf("connection opened with Transcontainer: %s", hostname))
			})

		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			log.WithFields(
				log.Fields{
					"component": "lifecycle",
					"lifecycleState": tc.State,
					"connectionId": tc.ConnectionId,
					"dataChannelId": d.ID(),
					"dataChannelLabel": d.Label(),
				}).Info("got new message: ", string(msg.Data))

			if gst.GLOBAL_STATE != string(msg.Data) {
				gst.GLOBAL_STATE = "switch_to_" + string(msg.Data)
			}
		})
	})

	tc.Connect(offer)
}

func (tc *Lifecycle) Connect(offer webrtc.SessionDescription) {

	// set state
	tc.State = CONNECT

	log.WithFields(
		log.Fields{
			"component": "lifecycle",
			"lifecycleState": tc.State,
			"connectionId": tc.ConnectionId,
		}).Info("setting peer connection local and remote descriptions.")


	// Set the remote SessionDescription
	if err := tc.PeerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := tc.PeerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(tc.PeerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = tc.PeerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

}

func (tc *Lifecycle) Stream(){

	// set state
	tc.State = ESTABLISHED

	// start reading RTCP
	tc.VideoJitter.StartRTCP()

	pipelineStr := fmt.Sprintf("souphttpsrc location=http://hyperscale.coldsnow.net:8080/bbb_360_abr.m3u8 ! hlsdemux ! decodebin3 name=demux caps=video/x-h264,stream-format=byte-stream ! appsink name=video demux. ! queue ! audioconvert ! audioresample ! opusenc ! appsink name=audio")
	log.WithFields(
		log.Fields{
			"component": "lifecycle",
			"lifecycleState": tc.State,
			"connectionId": tc.ConnectionId,
			"abrPipeline": pipelineStr,
		}).Info("setting abr pipeline.")

	tc.AbrPipeline = &gst.Pipeline{}
	tc.AbrPipeline = gst.CreatePipeline(pipelineStr, tc.AudioTrack, tc.VideoTrack, "abr")

	pipelineStrUI := fmt.Sprintf("souphttpsrc location=http://hyperscale.coldsnow.net:8080/bbb_360_ui.m3u8 ! hlsdemux ! decodebin3 name=demux caps=video/x-h264,stream-format=byte-stream ! appsink name=video demux. ! queue ! audioconvert ! audioresample ! opusenc ! appsink name=audio")
	log.WithFields(
		log.Fields{
			"component": "lifecycle",
			"lifecycleState": tc.State,
			"connectionId": tc.ConnectionId,
			"uiPipeline": pipelineStrUI,
		}).Info("setting ui pipeline.")


	tc.UiPipeLine = &gst.Pipeline{}
	tc.UiPipeLine = gst.CreatePipeline(pipelineStrUI, tc.AudioTrack, tc.VideoTrack, "ui")

	tc.AbrPipeline.Start()
	tc.UiPipeLine.Start()

	tc.State = STREAMING
	log.WithFields(
		log.Fields{
			"component": "lifecycle",
			"lifecycleState": tc.State,
			"connectionId": tc.ConnectionId,
		}).Info("starting both pipelines.")

}

func (tc *Lifecycle) Stop(){

	tc.State = STOP

	log.WithFields(
		log.Fields{
			"component": "lifecycle",
			"lifecycleState": tc.State,
			"connectionId": tc.ConnectionId,
		}).Info("stopping transcontainer lifecycle and starting again.")

	if tc.AbrPipeline != nil {
		tc.AbrPipeline.Pause()
	}
	if tc.UiPipeLine != nil {
		tc.UiPipeLine.Pause()
	}
	if tc.PeerConnection != nil {
		err := tc.PeerConnection.Close()
		if err != nil {
			panic(err)
		}
	}

	tc.VideoJitter.Close()

	tc = NewLifecycle(tc.SignalingClient)

	gst.ResetGlobalState()
}

func (tc *Lifecycle) Restart(){
	tc.Stop()
	tc.Start()
}

func getPayloadType(m webrtc.MediaEngine, codecType webrtc.RTPCodecType, codecName string) uint8 {
	for _, codec := range m.GetCodecsByKind(codecType) {
		if codec.Name == codecName {
			return codec.PayloadType
		}
	}
	panic(fmt.Sprintf("Remote peer does not support %s", codecName))
}
