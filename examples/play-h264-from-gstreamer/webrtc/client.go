package webrtc

import (
	"context"
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
	"reflect"
)

const SIGNALLING_RETRIES = 60

type WebRTCClient struct {
	State            	State
	clientConnectionId  string
	connectionId     	string
	signallingClient 	signalling.SignallingClient

	context				context.Context
	contextCancel 		context.CancelFunc

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

	if client.State == INITIATE {
		return
	}

	client.changeState(INITIATE)
	client.clientConnectionId = clientConnectionId

	log.WithFields(
		log.Fields{
			"component": 		      "webrtcclient",
			"state": 				  client.State,
			"clientConnectionId":     client.clientConnectionId,
		}).Debug("webrtc client connect.")

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
			client.context, client.contextCancel = client.signallingClient.GetAnswer(connectionId, SIGNALLING_RETRIES, client.handleAnswer)
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

		if client.OnMessageHandler != nil {
			client.OnMessageHandler(msg)
		}
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

func (client *WebRTCClient) handleAnswer(answer *webrtc.SessionDescription, err error){

	if client.context.Err() != nil {
		// context got canceled
		return
	}

	if err != nil {
		panic(err)
	}

	if err = client.peerConnection.SetRemoteDescription(*answer); err != nil {
		panic(err)
	}
}

func (client *WebRTCClient) Disconnect() {

	if client.State == IDLE {
		return
	}

	log.WithFields(
		log.Fields{
			"component": 		      "webrtcclient",
			"state": 				  client.State,
			"clientConnectionId":     client.clientConnectionId,
		}).Debug("webrtc client disconnect.")

	client.State = DISCONNECTED
	if client.contextCancel != nil {
		client.contextCancel()
		client.contextCancel = nil
	}
	if client.peerConnection != nil {
		client.peerConnection.Close()
	}
	client.connectionId = ""
	client.clientConnectionId = ""
	client.changeState(IDLE)
}

func (client *WebRTCClient) StartReadingRTPs(){

	log.WithFields(
		log.Fields{
			"component": 		      "webrtcclient",
			"state": 				  client.State,
			"clientConnectionId":     client.clientConnectionId,
		}).Debug("start reading rtps packets.")

	go func() {
		samplebuilder := samplebuilder.New(50, &codecs.H264Packet{})

		for {
			if client.State == DISCONNECTED || client.State == FAILED {
				// do not read if client connection is closed
				return
			} else if client.State != CONNECTED {
				// panic since we have a race condition
				panic("read rtp before connection is ready")
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
				// TODO: ignore?
				continue
			}
			samplebuilder.Push(packet)

			sample, _ := samplebuilder.PopWithTimestamp()
			if sample != nil {
				// end of sample
				sample.Samples = 90000/25
				client.OnSampleHandler(*sample, utils.UI, utils.VIDEO) // mark as ui video only
			}
		}
	}()
}

// WriteRTCP gets a RTCP packet from the TC, switches the SSRC and forwards to the UI
func (client *WebRTCClient) WriteRTCP(packet rtcp.Packet) error {

	log.WithFields(
		log.Fields{
			"component":          "webrtcclient",
			"state":              client.State,
			"clientConnectionId": client.clientConnectionId,
		}).Debugf("write rtcp packet %s", reflect.TypeOf(packet))

	if client.State != CONNECTED {
		return errors.New(fmt.Sprintf("write rtcp called while client state is set to %s", client.State))
	}

	var newPacket rtcp.Packet
	switch packet.(type) {

	case *rtcp.PictureLossIndication:
		newPliPacket := packet.(*rtcp.PictureLossIndication)
		newPliPacket.MediaSSRC = client.videoTrack.SSRC()
		newPacket = newPliPacket
		break
	case *rtcp.FullIntraRequest:
		newFIRPacket := packet.(*rtcp.FullIntraRequest)
		newFIRPacket.MediaSSRC = client.videoTrack.SSRC()
		newPacket = newFIRPacket
		break
	case *rtcp.ReceiverEstimatedMaximumBitrate:
		newREMBPacket := packet.(*rtcp.ReceiverEstimatedMaximumBitrate)
		newREMBPacket.SenderSSRC = client.videoTrack.SSRC()
		newPacket = newREMBPacket
		break
	case *rtcp.TransportLayerNack:
		newNackPacket := packet.(*rtcp.TransportLayerNack)
		newNackPacket.MediaSSRC = client.videoTrack.SSRC()
		newPacket = newNackPacket
		break
	case *rtcp.ReceiverReport:
		newRRPacket := packet.(*rtcp.ReceiverReport)
		newPacket = newRRPacket
		break
	default:
	}

	if newPacket != nil {
		errSend := client.peerConnection.WriteRTCP([]rtcp.Packet{newPacket})
		if errSend != nil {
			log.WithFields(
				log.Fields{
					"component":          "webrtcclient",
					"state":              client.State,
					"clientConnectionId": client.clientConnectionId,
					"ssrc":               client.videoTrack.SSRC(),
					"error":              errSend,
				}).Error("failed to send RTCP packet.")
			return errSend
		}
	}

	return nil
}

func (client *WebRTCClient) changeState(state State) {
	if state == client.State{
		return
	}

	log.WithFields(
		log.Fields{
			"component": 	"webrtcclient",
			"state": 		client.State,
			"newState": 	state,
			"connectionId": client.connectionId,
		}).Debug("connection state has changed.")

	client.State = state
	if client.OnStateChangeHandler != nil {
		client.OnStateChangeHandler(state)
	}
}

func (client *WebRTCClient) SendDataMessage(message string) error{

	if client.State != CONNECTED{
		return errors.New(fmt.Sprintf("trying to send message while client state is set to %s", client.State))
	}

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
