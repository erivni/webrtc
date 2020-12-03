package webrtc

import (
	"fmt"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	signalling "github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/signallingclient"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/utils"
	"github.com/pion/webrtc/v3/pkg/media"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

const SIGNALLING_RETRIES = 15

type WebRTCClient struct {
	State            State
	readingRTP		 bool
	connectionId     string
	signallingClient signalling.SignallingClient

	peerConnection 				*webrtc.PeerConnection
	videoTrack					*webrtc.Track

	OnMessageHandler func(webrtc.DataChannelMessage)
	OnStateChangeHandler func(State)
	OnSampleHandler func(media.Sample, utils.StreamType, utils.SampleType)
	OnRTPHandler func(packet *rtp.Packet)
}

func NewWebRTCClient(signallingClient signalling.SignallingClient, onMessageHandler func(webrtc.DataChannelMessage), onStateChangeHandler func(State), OnSampleHandler func(media.Sample, utils.StreamType, utils.SampleType)) *WebRTCClient {
	return &WebRTCClient{State: IDLE, signallingClient: signallingClient, OnMessageHandler: onMessageHandler, OnStateChangeHandler: onStateChangeHandler, OnSampleHandler: OnSampleHandler}
}

func (client *WebRTCClient) Connect(connectionId string) {
	client.changeState(INITIATE)

	mediaEngine := webrtc.MediaEngine{}
	mediaEngine.RegisterCodec(webrtc.NewRTPH264Codec(102, 90000))
	mediaEngine.RegisterCodec(webrtc.NewRTPOpusCodec(111, 48000))

	settingEngine := webrtc.SettingEngine{}
	err := settingEngine.SetEphemeralUDPPortRange(50003, 50005)
	if err != nil {
		panic(err)
	}

	// Create a new RTCPeerConnection
	var api = webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithSettingEngine(settingEngine),
	)
	client.peerConnection, err = api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	// create Transceivers
	if _, err = client.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}
	if _, err = client.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	// create datachannel
	dataChannel, err := client.peerConnection.CreateDataChannel("hyperscale", nil)
	if err != nil {
		panic(err)
	}

	// events registration
	client.peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {

		// TODO: send RTCP
		log.WithFields(
			log.Fields{
				"component": 		"webrtcclient",
				"state": 			client.State,
				"connectionId":     client.connectionId,
				"payloadType":   track.PayloadType(),
				"codec":   track.Codec().Name,
			}).Debug("ui track has started.")

		client.videoTrack = track
		client.changeState(CONNECTED)
	})

	client.peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.WithFields(
			log.Fields{
				"component": 		"webrtcclient",
				"state": 			client.State,
				"connectionId":     client.connectionId,
				"connectionState": connectionState.String(),
			}).Debug("connection state has changed.")

		if connectionState == webrtc.ICEConnectionStateConnected {
		} else if connectionState == webrtc.ICEConnectionStateFailed {
			client.changeState(FAILED)
		} else if connectionState == webrtc.ICEConnectionStateDisconnected {
			client.changeState(DISCONNECTED)
		}
	})

	client.peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			log.WithFields(
				log.Fields{
					"component": 		"webrtcclient",
					"state": 			client.State,
					"connectionId":     client.connectionId,
				}).Debug("no more ice candidates, posting an offer to signaling server.")

			// TODO: change to new API
			connectionId, err := client.signallingClient.SendOffer(*client.peerConnection.LocalDescription())
			if err != nil {
				panic(err)
			}

			answer, err := client.signallingClient.GetAnswer(connectionId, SIGNALLING_RETRIES)
			if err != nil {
				panic(err)
			}

			if err = client.peerConnection.SetRemoteDescription(*answer); err != nil {
				panic(err)
			}
		}
	})

	dataChannel.OnOpen(func() {
		log.WithFields(
			log.Fields{
				"component": 		"webrtcclient",
				"state": 			client.State,
				"connectionId":     client.connectionId,
				"dataChannelId":    dataChannel.ID(),
				"dataChannelLabel": dataChannel.Label(),
			}).Debug("data channel is open.")

		hostname, _ := os.Hostname()
		dataChannel.SendText(fmt.Sprintf("connection opened with Transcontainer: %s", hostname))
	})

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.WithFields(
			log.Fields{
				"component": 		"webrtcclient",
				"state": 			client.State,
				"connectionId":     client.connectionId,
				"dataChannelId":    dataChannel.ID(),
				"dataChannelLabel": dataChannel.Label(),
			}).Debug("got new message: ", string(msg.Data))
	})

	// creating an offer
	offer, err := client.peerConnection.CreateOffer(nil)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": 		"webrtcclient",
				"state": 			client.State,
				"connectionId":     client.connectionId,
				"error": err.Error(),
			}).Error("failed to create an offer.")
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(client.peerConnection)

	// Set the local SessionDescription
	if err := client.peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

}

func (client *WebRTCClient) Disconnect() {
	client.peerConnection.Close()
	client.changeState(IDLE)
	client.connectionId = ""
}

func (client *WebRTCClient) StartReadingRTPs(){
	go func() {
		client.readingRTP = true
		//samplebuilder := samplebuilder.New(50, &codecs.H264Packet{})

		for {
			if client.State != CONNECTED {
				panic("read rtp before connection is ready")
			}

			if ! client.readingRTP {
				return
			}

			// Read RTP packets being sent to Pion
			packet, err := client.videoTrack.ReadRTP()
			if err != nil {
				log.WithFields(
					log.Fields{
						"component":    "webrtcclient",
						"state":        client.State,
						"connectionId": client.connectionId,
						"ssrc":         client.videoTrack.SSRC(),
						"error":        err,
					}).Error("failed to read RTP packet.")
			}

			client.OnRTPHandler(packet)
			//sample := samplebuilder.Pop()
			//if sample != nil {
			//	// end of sample
			//	sample.Samples = 90000/25
			//	client.OnSampleHandler(*sample, utils.UI, utils.VIDEO) // TODO: mark as ui video only
			//}
			//
			//samplebuilder.Push(packet)
		}
	}()
}

func (client *WebRTCClient) StopReadingRTP(){
	client.readingRTP = false
}

func (client *WebRTCClient) WriteRTCP() {

	go func() {
		ticker := time.NewTicker(time.Second * 3)
		for range ticker.C {
			if client.State != CONNECTED {
				return
			}

			errSend := client.peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: client.videoTrack.SSRC()}})
			if errSend != nil {
				log.WithFields(
					log.Fields{
						"component": 		"webrtcclient",
						"state": 			client.State,
						"connectionId":     client.connectionId,
						"ssrc":   			client.videoTrack.SSRC(),
						"error":   			errSend,
					}).Error("failed to send RTCP packet.")
			}
		}
	}()

}

func (client *WebRTCClient) changeState(state State) {
	client.State = state
	if client.OnStateChangeHandler != nil {
		client.OnStateChangeHandler(state)
	}
}

func (client *WebRTCClient) SetListeners(onMessageHandler func(webrtc.DataChannelMessage), onSampleHandler func(media.Sample, utils.StreamType, utils.SampleType), onRTPHandler func(*rtp.Packet)) {
	client.OnMessageHandler = onMessageHandler
	client.OnSampleHandler = onSampleHandler
	client.OnRTPHandler = onRTPHandler
}
