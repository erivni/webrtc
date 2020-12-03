package webrtc

import (
	"fmt"
	"github.com/pion/randutil"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3"
	signalling "github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/signallingclient"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/utils"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/webrtc/rtpbuffer"
	"github.com/pion/webrtc/v3/pkg/media"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

type WebRTCServer struct {
	State            State
	ConnectionId     string
	signallingClient signalling.SignallingClient

	peerConnection 	*webrtc.PeerConnection
	audioTrack		*webrtc.Track
	videoTrack		*webrtc.Track

	videoJitter 	*rtpbuffer.Jitter

	Interleaved		bool

	OnMessageHandler func(webrtc.DataChannelMessage)
	OnStateChangeHandler func(State)
	OnRTCPHandler func([]rtcp.Packet)
}

func NewWebRTCServer(signallingClient signalling.SignallingClient, onMessageHandler func(webrtc.DataChannelMessage), onStateChangeHandler func(State), onRTCPHandler func([]rtcp.Packet)) *WebRTCServer {
	return &WebRTCServer{State: IDLE, Interleaved: false, signallingClient: signallingClient, OnMessageHandler: onMessageHandler, OnStateChangeHandler: onStateChangeHandler, OnRTCPHandler: onRTCPHandler}
}


func (server *WebRTCServer) Connect() {
	server.changeState(INITIATE)

	var err error
	server.ConnectionId, err = server.signallingClient.GetQueue()
	if err != nil {
		panic(err)
	}

	offer, err := server.signallingClient.GetOffer(server.ConnectionId)
	if err != nil {
		panic(err)
	}

	log.WithFields(
		log.Fields{
			"component": "webrtcserver",
			"state": server.State,
			"connectionId": server.ConnectionId,
		}).Debug("got a offer, trying to connect to peer.")

	mediaEngine := webrtc.MediaEngine{}
	err = mediaEngine.PopulateFromSDP(*offer)
	if err != nil {
		panic(err)
	}

	settingEngine := webrtc.SettingEngine{}
	err = settingEngine.SetEphemeralUDPPortRange(50000, 50002)
	if err != nil {
		panic(err)
	}

	var api = webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithSettingEngine(settingEngine),
	)

	server.peerConnection, err = api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	server.videoTrack, err = server.peerConnection.NewTrack(getPayloadType(mediaEngine, webrtc.RTPCodecTypeVideo, "H264"), randutil.NewMathRandomGenerator().Uint32(), "video", "transcontainer")
	if _, err = server.peerConnection.AddTrack(server.videoTrack); err != nil {
		panic(err)
	}

	server.audioTrack, err = server.peerConnection.NewTrack(getPayloadType(mediaEngine, webrtc.RTPCodecTypeAudio, "opus"), randutil.NewMathRandomGenerator().Uint32(), "audio", "transcontainer")
	if _, err = server.peerConnection.AddTrack(server.audioTrack); err != nil {
		panic(err)
	}

	server.videoJitter = rtpbuffer.NewJitter(server.peerConnection, server.videoTrack, strings.ToLower(os.Getenv("FORWARD_RTP")) == "true")

	// set Interleaved
	_, ok := server.videoTrack.Codec().Payloader.(*codecs.H264InterleavedPayloader)
	if ok {
		server.Interleaved = true
	}

	log.WithFields(
		log.Fields{
			"component": "webrtcserver",
			"state": server.State,
			"connectionId": server.ConnectionId,
		}).Infof("connection is set with interleaved equals %t", server.Interleaved)

	// events registration
	server.peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.WithFields(
			log.Fields{
				"component": "webrtcserver",
				"state": server.State,
				"connectionId": server.ConnectionId,
			}).Debug("connection state has changed.")

		if connectionState == webrtc.ICEConnectionStateConnected {
			server.changeState(CONNECTED)
			server.videoJitter.StartRTCP(server.OnRTCPHandler)
		} else if connectionState == webrtc.ICEConnectionStateFailed {
			server.changeState(FAILED)
		} else if connectionState == webrtc.ICEConnectionStateDisconnected {
			server.changeState(DISCONNECTED)
		}
	})

	server.peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			log.WithFields(
				log.Fields{
					"component": "webrtcserver",
					"state": server.State,
					"connectionId": server.ConnectionId,
				}).Debug("no more ice candidates, posting answer to signaling server.")

			err = server.signallingClient.SendAnswer(server.ConnectionId, *server.peerConnection.LocalDescription())
			if err != nil {
				panic(err)
			}
		}
	})

	server.peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		log.WithFields(
			log.Fields{
				"component": "webrtcserver",
				"state": server.State,
				"connectionId": server.ConnectionId,
				"dataChannelId":    d.ID(),
				"dataChannelLabel": d.Label(),
			}).Debug("new data channel.")

		d.OnOpen(func() {
			log.WithFields(
				log.Fields{
					"component": "webrtcserver",
					"state": server.State,
					"connectionId": server.ConnectionId,
					"dataChannelId":    d.ID(),
					"dataChannelLabel": d.Label(),
				}).Debug("data channel is open.")

			hostname, _ := os.Hostname()
			d.SendText(fmt.Sprintf("connection opened with Transcontainer: %s", hostname))
		})

		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			log.WithFields(
				log.Fields{
					"component": "webrtcserver",
					"state": server.State,
					"connectionId": server.ConnectionId,
					"dataChannelId":    d.ID(),
					"dataChannelLabel": d.Label(),
				}).Debug("got new message: ", string(msg.Data))

			server.OnMessageHandler(msg)
		})
	})

	// Set the remote SessionDescription
	if err := server.peerConnection.SetRemoteDescription(*offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := server.peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(server.peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = server.peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

}

func (server *WebRTCServer) Disconnect() {
	server.peerConnection.Close()
	server.changeState(IDLE)
	server.ConnectionId = ""
}

func (server *WebRTCServer) WriteRTP(packet *rtp.Packet) {
	if server.State != CONNECTED {
		if server.State == DISCONNECTED {
			// TODO: dropping rtps on disconnect and not don't panic
			return
		}
		panic("sending RTPs on a close connection")
	}

	packet.SSRC = server.videoTrack.SSRC()
	server.videoJitter.WriteRTP(packet)
}

func (server *WebRTCServer) WriteSample(sample media.Sample, sampleType utils.SampleType) {
	if server.State != CONNECTED {
		if server.State == DISCONNECTED {
			// TODO: dropping rtps on disconnect and not don't panic
			return
		}
		panic("sending Sample before connection is established")
	}

	if sampleType == utils.VIDEO {
		server.videoJitter.WriteSample(sample)
	} else {
		server.audioTrack.WriteSample(sample)
	}
}

func (server *WebRTCServer) changeState(state State) {
	server.State = state
	if server.OnStateChangeHandler != nil {
		server.OnStateChangeHandler(state)
	}
}

func (server *WebRTCServer) SetListeners(onMessageHandler func(webrtc.DataChannelMessage), OnRTCPHandler func([]rtcp.Packet)) {
	server.OnMessageHandler = onMessageHandler
	server.OnRTCPHandler = OnRTCPHandler
}

func getPayloadType(m webrtc.MediaEngine, codecType webrtc.RTPCodecType, codecName string) uint8 {
	for _, codec := range m.GetCodecsByKind(codecType) {
		if codec.Name == codecName {
			return codec.PayloadType
		}
	}
	panic(fmt.Sprintf("Remote peer does not support %s", codecName))
}
