package webrtc

import (
	"errors"
	"fmt"
	"github.com/pion/rtcp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3"
	signalling "github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/signallingclient"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/utils"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/samplebuilder"
	log "github.com/sirupsen/logrus"
	"os"
)

const SIGNALLING_RETRIES = 15

type WebRTCClient struct {
	State            	State
	readingRTP		 	bool
	clientConnectionId  string
	connectionId     	string
	signallingClient 	signalling.SignallingClient

	peerConnection 		*webrtc.PeerConnection
	videoTrack			*webrtc.Track

	dataChannel         *webrtc.DataChannel
	OnMessageHandler func(webrtc.DataChannelMessage)
	OnStateChangeHandler func(State)
	OnSampleHandler func(media.Sample, utils.StreamType, utils.SampleType)
}

func NewWebRTCClient(signallingClient signalling.SignallingClient, onMessageHandler func(webrtc.DataChannelMessage), onStateChangeHandler func(State), OnSampleHandler func(media.Sample, utils.StreamType, utils.SampleType)) *WebRTCClient {
	return &WebRTCClient{State: IDLE, signallingClient: signallingClient, OnMessageHandler: onMessageHandler, OnStateChangeHandler: onStateChangeHandler, OnSampleHandler: OnSampleHandler}
}

func (client *WebRTCClient) Connect(clientConnectionId string) {
	client.changeState(INITIATE)
	client.clientConnectionId = clientConnectionId
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
	client.dataChannel, err = client.peerConnection.CreateDataChannel("hyperscale", nil)
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
				"clientConnectionId":     client.clientConnectionId,
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
				"clientConnectionId":     client.clientConnectionId,
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
					"clientConnectionId":     client.clientConnectionId,
				}).Debug("no more ice candidates, posting an offer to signaling server.")

			connectionId, err := client.signallingClient.SendOffer(*client.peerConnection.LocalDescription(), client.clientConnectionId)
			if err != nil {
				panic(err)
			}

			client.connectionId = connectionId

			answer, err := client.signallingClient.GetAnswer(connectionId, SIGNALLING_RETRIES)
			if err != nil {
				panic(err)
			}

			if err = client.peerConnection.SetRemoteDescription(*answer); err != nil {
				panic(err)
			}
		}
	})

	client.dataChannel.OnOpen(func() {
		log.WithFields(
			log.Fields{
				"component": 		"webrtcclient",
				"state": 			client.State,
				"clientConnectionId":     client.clientConnectionId,
				"dataChannelId":    client.dataChannel.ID(),
				"dataChannelLabel": client.dataChannel.Label(),
			}).Debug("data channel is open.")

		hostname, _ := os.Hostname()
		client.dataChannel.SendText(fmt.Sprintf("connection opened with Transcontainer: %s", hostname))
	})

	client.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.WithFields(
			log.Fields{
				"component": 		"webrtcclient",
				"state": 			client.State,
				"clientConnectionId":     client.clientConnectionId,
				"dataChannelId":    client.dataChannel.ID(),
				"dataChannelLabel": client.dataChannel.Label(),
			}).Debug("got new message: ", string(msg.Data))
	})

	// creating an offer
	offer, err := client.peerConnection.CreateOffer(nil)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": 		"webrtcclient",
				"state": 			client.State,
				"clientConnectionId":     client.clientConnectionId,
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
	if client.peerConnection == nil{
		return
	}
	client.peerConnection.Close()
	client.changeState(IDLE)
	client.connectionId = ""
	client.clientConnectionId = ""
}

func (client *WebRTCClient) StartReadingRTPs(){
	go func() {
		client.readingRTP = true
		samplebuilder := samplebuilder.New(50, &codecs.H264Packet{})

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
						"clientConnectionId": client.clientConnectionId,
						"ssrc":         client.videoTrack.SSRC(),
						"error":        err,
					}).Error("failed to read RTP packet.")
			}
			samplebuilder.Push(packet)

			sample, _ := samplebuilder.PopWithTimestamp()
			if sample != nil {
				// end of sample
				sample.Samples = 90000/25
				client.OnSampleHandler(*sample, utils.UI, utils.VIDEO) // TODO: mark as ui video only
			}
		}
	}()
}

func (client *WebRTCClient) StopReadingRTP(){
	client.readingRTP = false
}

// WriteRTCP gets a RTCP packet from the TC, switches the SSRC and forwards to the UI
func (client *WebRTCClient) WriteRTCP(packet rtcp.Packet) {
	if client.State != CONNECTED {
		panic("write rtcp called while client state is not connected")
	}
	var newPacket rtcp.Packet
	switch packet.(type) {

	case *rtcp.PictureLossIndication:
		newPliPacket := packet.(*rtcp.PictureLossIndication)
		newPliPacket.MediaSSRC = client.videoTrack.SSRC()
		newPacket = newPliPacket
	case *rtcp.FullIntraRequest:
		newFIRPacket := packet.(*rtcp.FullIntraRequest)
		newFIRPacket.MediaSSRC = client.videoTrack.SSRC()
		newPacket = newFIRPacket
	case *rtcp.ReceiverEstimatedMaximumBitrate:
		newREMBPacket := packet.(*rtcp.ReceiverEstimatedMaximumBitrate)
		newREMBPacket.SenderSSRC = client.videoTrack.SSRC()
		newPacket = newREMBPacket
	case *rtcp.TransportLayerNack:
		newNackPacket := packet.(*rtcp.TransportLayerNack)
		newNackPacket.MediaSSRC = client.videoTrack.SSRC()
		newPacket = newNackPacket
	case *rtcp.ReceiverReport:
		newRRPacket := packet.(*rtcp.ReceiverReport)
		newPacket = newRRPacket
	default:
	}

	errSend := client.peerConnection.WriteRTCP([]rtcp.Packet{newPacket})
	if errSend != nil {
		log.WithFields(
			log.Fields{
				"component": 		"webrtcclient",
				"state": 			client.State,
				"clientConnectionId":     client.clientConnectionId,
				"ssrc":   			client.videoTrack.SSRC(),
				"error":   			errSend,
			}).Error("failed to send RTCP packet.")
	}
}

func (client *WebRTCClient) changeState(state State) {
	if state == client.State{
		return
	}
	client.State = state
	if client.OnStateChangeHandler != nil {
		client.OnStateChangeHandler(state)
	}
}

func (client *WebRTCClient) SetListeners(onMessageHandler func(webrtc.DataChannelMessage), onSampleHandler func(media.Sample, utils.StreamType, utils.SampleType)) {
	client.OnMessageHandler = onMessageHandler
	client.OnSampleHandler = onSampleHandler
}

func (client *WebRTCClient) SendDataMessage(message string) error{
	errorMessage := ""
	if client.dataChannel.ReadyState() != webrtc.DataChannelStateOpen{
		errorMessage = "cannot send data message. data channel is in " + string(client.dataChannel.ReadyState()) + " state"
		log.WithFields(
			log.Fields{
				"component": 		"webrtcclient",
				"state": 			client.State,
				"dataChannelState": client.dataChannel.ReadyState(),
				"clientConnectionId":     client.clientConnectionId,
				"dataChannelMessage": message,
			}).Error(errorMessage)
		return errors.New(errorMessage)
	}
	err := client.dataChannel.SendText(message)
	if err != nil{
		errorMessage = "error while sending data message: " + err.Error()
		log.WithFields(
			log.Fields{
				"component": 		"webrtcclient",
				"state": 			client.State,
				"dataChannelState": client.dataChannel.ReadyState(),
				"clientConnectionId":     client.clientConnectionId,
				"dataChannelMessage": message,
			}).Error(errorMessage)
		return errors.New(errorMessage)
	}
	return nil
}