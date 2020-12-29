package transcontainer

import "C"
import (
	"encoding/binary"
	"fmt"
	"github.com/pion/rtcp"
	pion "github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/gst"
	signalling "github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/signallingclient"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/utils"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/webrtc"
	"github.com/pion/webrtc/v3/pkg/media"
	log "github.com/sirupsen/logrus"
	"reflect"
	"strings"
)

const (
	maxDonValue = ^uint16(0)
	donSize = 2
)

type StreamingState uint8
const (
	UI            StreamingState = 0
	ABR           StreamingState = 1
	SWITCH_TO_UI  StreamingState = 2
	SWITCH_TO_ABR StreamingState = 3
	RESIZE_UP	  StreamingState = 4
	RESIZE_DOWN	  StreamingState = 5
	TUNE_TO		  StreamingState = 6
)
func (s StreamingState) String() string {
	switch s {
	case UI:
		return "UI"
	case ABR:
		return "ABR"
	case SWITCH_TO_UI:
		return "SWITCH_TO_UI"
	case SWITCH_TO_ABR:
		return "SWITCH_TO_ABR"
	case RESIZE_UP:
		return "RESIZE_UP"
	case RESIZE_DOWN:
		return "RESIZE_DOWN"
	case TUNE_TO:
		return "TUNE_TO"
	default:
		return fmt.Sprintf("%d", int(s))
	}
}


type Transcontainer struct {
	State State
	StreamingState

	// TODO: do we use it as a sessionid
	//connectionId string

	uiConnection *webrtc.WebRTCClient
	clientConnection *webrtc.WebRTCServer

	abrPlayer *gst.Pipeline

	don 	  uint16
	OnStateChangeHandler func(State)
	OnStreamingStateChangeHandler func(StreamingState)
}

func NewTranscontainer(signallingClient signalling.SignallingClient, onStateChangeHandler func(State), OnStreamingStateChangeHandler func(StreamingState)) *Transcontainer {
	t := &Transcontainer{
		State:                STOP,
		StreamingState:       UI,
		don:				  0,
		OnStateChangeHandler: onStateChangeHandler,
		OnStreamingStateChangeHandler: OnStreamingStateChangeHandler,
	}

	t.uiConnection = webrtc.NewWebRTCClient(signallingClient, t.processUiMessage, t.processUiStateChange, t.processSample )
	t.clientConnection = webrtc.NewWebRTCServer(signallingClient, t.processClientMessage, t.processClientStateChange, t.processRTCP)

	return t
}

func (t *Transcontainer) Start() {

	if t.State == START {
		return
	}

	log.WithFields(
		log.Fields{
			"component":      "transcontainer",
			"state":          t.State,
			"streamingState": t.StreamingState,
		}).Info("starting transcontainer..")

	t.changeState(START)

	// waiting to serve clients
	t.clientConnection.Connect()
}

func (t *Transcontainer) Stream() {

	if t.State == STREAM{
		return
	}

	log.WithFields(
		log.Fields{
			"component": 		"transcontainer",
			"state":			t.State,
			"streamingState":	t.StreamingState,
		}).Info("start streaming transcontainer")

	t.don = 0

	// start reading from ui connection
	t.uiConnection.StartReadingRTPs()

	pipelineStr := fmt.Sprintf("souphttpsrc location=http://hyperscale.coldsnow.net:8080/bbb_360_abr.m3u8 ! hlsdemux ! decodebin3 name=demux caps=video/x-h264,stream-format=byte-stream ! appsink name=video demux. ! queue ! audioconvert ! audioresample ! opusenc ! appsink name=audio")
	log.WithFields(
		log.Fields{
			"component": "lifecycle",
			"state"	: 	 "start",
			"abrPlayer": pipelineStr,
		}).Debug("setting abrPlayer.")

	t.abrPlayer = gst.CreatePipeline(pipelineStr, nil, nil, "abr", t.processSample)
	t.abrPlayer.Start()

	log.WithFields(
                log.Fields{
                        "component": "lifecycle",
                        "state" :        "start",
                }).Info("sending pli..")

        t.uiConnection.WriteRTCP(&rtcp.PictureLossIndication{})
	t.changeState(STREAM)
}

func (t *Transcontainer) Stop() {

	if t.State == STOP {
		return
	}

	log.WithFields(
		log.Fields{
			"component": 		"transcontainer",
			"state":			t.State,
			"streamingState":	t.StreamingState,
		}).Info("stopping transcontainer...")

	t.uiConnection.Disconnect()
	t.clientConnection.Disconnect()
	if t.abrPlayer != nil{
		t.abrPlayer.Stop()
	}

	t.changeState(STOP)
	t.changeStreamingState(UI)
}

// ProcessRTCP receives RTCP messages from the client connection and makes decisions based on packet type and state
// e.g. when in UI we can decide to lower bitrate. when in ABR we can decide to lock on a different ABR bitrate
func (t *Transcontainer) processRTCP(packet rtcp.Packet){

	if t.State != STREAM {
		log.WithFields(
			log.Fields{
				"component": 		"transcontainer",
				"state": 			t.State,
				"steamingState": 	t.StreamingState,
			}).Warnf("processRTCP called while transcontainer state is %s. ignoring packet..", t.State)
		return
	}

	log.WithFields(
		log.Fields{
			"component": 		"transcontainer",
			"state": 			t.State,
			"steamingState": 	t.StreamingState,
		}).Tracef("process %s rtcp packet ", reflect.TypeOf(packet))

	switch packet := packet.(type) {
	case *rtcp.PictureLossIndication:
		// if streaming mode == ui then forward this PLI to the UI
		if t.StreamingState == UI || t.StreamingState == SWITCH_TO_UI {
			t.uiConnection.WriteRTCP(packet)
		}

		// TODO: handle when streaming ABR
		break
	case *rtcp.FullIntraRequest:
		// if streaming mode == ui then forward this PLI to the UI
		if t.StreamingState == UI || t.StreamingState == SWITCH_TO_UI {
			t.uiConnection.WriteRTCP(packet)
		}

		// TODO: handle when streaming ABR
		break
	case *rtcp.ReceiverEstimatedMaximumBitrate:
	case *rtcp.TransportLayerNack:
	case *rtcp.ReceiverReport:
	default:

	}
}

func (t *Transcontainer) processSample(sample media.Sample, streamType utils.StreamType, sampleType utils.SampleType){

	if t.State != STREAM {
		log.WithFields(
			log.Fields{
				"component": 		"transcontainer",
				"state": 			t.State,
				"steamingState": 	t.StreamingState,
			}).Warnf("processSample called while transcontainer state is %s. ignoring packet..", t.State)
		return
	}

	if streamType == utils.UI && t.StreamingState == SWITCH_TO_UI {
		if sampleType == utils.VIDEO && isIframe(sample.Data) {
			t.changeStreamingState(UI)
		}
	} else if streamType == utils.ABR && t.StreamingState == SWITCH_TO_ABR {
		if sampleType == utils.VIDEO && isIframe(sample.Data) {
			t.changeStreamingState(ABR)
		}
	}
	if streamType == utils.ABR && (t.StreamingState == UI || t.StreamingState == SWITCH_TO_ABR) || streamType == utils.UI && (t.StreamingState == ABR || t.StreamingState == SWITCH_TO_UI) {
		return
	}

	log.WithFields(
		log.Fields{
			"component": 		"transcontainer",
			"state": 			t.State,
			"steamingState": 	t.StreamingState,
			"streamType":		streamType,
			"sampleType":		sampleType,
			"interleaved": 		t.clientConnection.Interleaved,
		}).Trace("sending sample...")

	if t.clientConnection.Interleaved && sampleType == utils.VIDEO {
		frame := make([]byte, donSize + len(sample.Data))
		binary.BigEndian.PutUint16(frame[:2], t.don)
		copy(frame[donSize:], sample.Data)

		t.clientConnection.WriteSample(media.Sample{Data: frame, Samples: sample.Samples}, sampleType)
		t.don = (t.don+1) % maxDonValue

	} else {
		t.clientConnection.WriteSample(sample, sampleType)
	}
}

func (t *Transcontainer) processUiMessage(msg pion.DataChannelMessage){

	if t.State != STREAM {
		log.WithFields(
			log.Fields{
				"component": 		"transcontainer",
				"state": 			t.State,
				"steamingState": 	t.StreamingState,
			}).Warnf("processUiMessage called while transcontainer state is %s. ignoring packet..", t.State)
		return
	}

	log.WithFields(
		log.Fields{
			"component": "transcontainer",
			"state": 			t.State,
			"steamingState": 	t.StreamingState,
			"datachannelMsg": 	string(msg.Data),
		}).Info("got message from ui.")

	t.processMessage(string(msg.Data))
}

func (t *Transcontainer) processClientMessage(msg pion.DataChannelMessage){

	if t.State != STREAM {
		log.WithFields(
			log.Fields{
				"component": 		"transcontainer",
				"state": 			t.State,
				"steamingState": 	t.StreamingState,
			}).Warnf("processClientMessage called while transcontainer state is %s. ignoring packet..", t.State)
		return
	}

	message := string(msg.Data)
	log.WithFields(
		log.Fields{
			"component": "transcontainer",
			"state": 			t.State,
			"streamingState": 	t.StreamingState,
			"datachannelMsg": message,
		}).Info("got message from client. will pass to UI")

	// pass the received data message to the ui
	t.uiConnection.SendDataMessage(message)

	t.processMessage(message)
}

func (t *Transcontainer) changeState(state State) {

	if state == t.State{
		return
	}

	log.WithFields(
		log.Fields{
			"component": 	"transcontainer",
			"state":  		t.State,
			"newState":     state,
		}).Debug("transcontainer changed state.")
	t.State = state
	if t.OnStateChangeHandler != nil {
		t.OnStateChangeHandler(state)
	}
}

func (t *Transcontainer) changeStreamingState(state StreamingState) {
	if state == t.StreamingState{
		return
	}

	t.StreamingState = state

	log.WithFields(
		log.Fields{
			"component": 		"transcontainer",
			"state":			t.State,
			"streamingState":	t.StreamingState,
		}).Info("changed streaming state...")

	if t.OnStreamingStateChangeHandler != nil {
		t.OnStreamingStateChangeHandler(state)
	}
}

func (t *Transcontainer) processMessage(message string){

	parsedMessage := strings.Split(message, " ")
	msgState := convertStringToTranscontainerState(parsedMessage[0])
	switch msgState {
	case ABR: {
		if t.StreamingState != msgState {
			t.changeStreamingState(SWITCH_TO_ABR)
		}
		break;
	}
	case UI:
		{
			if t.StreamingState != msgState {
				t.changeStreamingState(SWITCH_TO_UI)
				// force ui to send iframe as quickly as possible to have a quick switch
				t.uiConnection.WriteRTCP(&rtcp.PictureLossIndication{})
			}
			break;
		}
	case RESIZE_UP:
		t.uiConnection.WriteRTCP(&rtcp.ReceiverEstimatedMaximumBitrate{Bitrate: 10000000})
		break
	case RESIZE_DOWN:
		t.uiConnection.WriteRTCP(&rtcp.ReceiverEstimatedMaximumBitrate{Bitrate: 1000})
		break
	case TUNE_TO:
		break
	default:
		log.WithFields(
			log.Fields{
				"component":	"transcontainer",
				"state": 			t.State,
				"steamingState": 	t.StreamingState,
			}).Error("got unknown message: ", message)
		break;
	}

}

func (t *Transcontainer) processClientStateChange(state webrtc.State) {
	log.WithFields(
		log.Fields{
			"component": 			"transcontainer",
			"state":				 t.State,
			"uiConnectionState":	 t.uiConnection.State,
			"clientConnectionState": t.clientConnection.State,
			"newClientConnectionState":		state,
		}).Debug("client connection state changed")

	switch state {
	case webrtc.INITIATE:
		log.WithFields(
			log.Fields{
				"component": 			"transcontainer",
				"state":				 t.State,
				"uiConnectionState":	 t.uiConnection.State,
				"clientConnectionState": t.clientConnection.State,
				"newClientConnectionState":		state,
			}).Info("client has a waiting offer, staring ui connection")
		t.uiConnection.Connect(t.clientConnection.ConnectionId)
		break
	case webrtc.CONNECTED:
		if t.uiConnection.State == webrtc.CONNECTED && t.State != STREAM {
			log.WithFields(
				log.Fields{
					"component": 			"transcontainer",
					"state":				 t.State,
					"uiConnectionState":	 t.uiConnection.State,
					"clientConnectionState": t.clientConnection.State,
					"newClientConnectionState":		state,
				}).Info("client and ui connections are ready, starting transcontainer")
			t.Stream()
		}
		break
	case webrtc.FAILED, webrtc.DISCONNECTED: // being called by the webrtc
		t.Stop()
		break
	case webrtc.IDLE: // should be called only on client.Disconnect
		break
	default:
		break
	}
}

func (t *Transcontainer) processUiStateChange(state webrtc.State) {

	log.WithFields(
		log.Fields{
			"component": 			"transcontainer",
			"state":				 t.State,
			"uiConnectionState":	 t.uiConnection.State,
			"clientConnectionState": t.clientConnection.State,
			"newUiConnectionState":	 state,
		}).Debug("ui connection changed")

	switch state {
	case webrtc.CONNECTED:
		if t.clientConnection.State == webrtc.CONNECTED && t.State != STREAM {
			log.WithFields(
				log.Fields{
					"component": 			"transcontainer",
					"state":				 t.State,
					"uiConnectionState":	 t.uiConnection.State,
					"clientConnectionState": t.clientConnection.State,
					"newUiConnectionState":	 state,
				}).Info("client and ui connections are ready, starting transcontainer and abrPlayer")
			t.Stream()
		}
		break
	case webrtc.FAILED, webrtc.DISCONNECTED: // being called by the webrtc
		t.Stop()
		break
	case webrtc.INITIATE, webrtc.IDLE: // should be called only on ui.Disconnect
		break
	default:
		break
	}
}

func convertStringToTranscontainerState(state string) StreamingState{
	switch state {
	case "ui":
		return UI
	case "abr":
		return ABR
	case "switch_to_ui":
		return SWITCH_TO_UI
	case "switch_to_abr":
		return SWITCH_TO_ABR
	case "resize_up":
		return RESIZE_UP
	case "resize_down":
		return RESIZE_DOWN
	default:
		return UI
	}
}

func isIframe(buffer []byte) bool{
	isIframe := false
	emitNalus(buffer, func(nalu []byte) {
		naluType := nalu[0] & naluTypeBitmask
		if naluType == 5 {
			isIframe = true
		}
	})
	return isIframe
}

func emitNalus(nals []byte, emit func([]byte)) {
	nextInd := func(nalu []byte, start int) (indStart int, indLen int) {
		zeroCount := 0

		for i, b := range nalu[start:] {
			if b == 0 {
				zeroCount++
				continue
			} else if b == 1 {
				if zeroCount >= 2 {
					return start + i - zeroCount, zeroCount + 1
				}
			}
			zeroCount = 0
		}
		return -1, -1
	}

	nextIndStart, nextIndLen := nextInd(nals, 0)
	if nextIndStart == -1 {
		emit(nals)
	} else {
		for nextIndStart != -1 {
			prevStart := nextIndStart + nextIndLen
			nextIndStart, nextIndLen = nextInd(nals, prevStart)
			if nextIndStart != -1 {
				emit(nals[prevStart:nextIndStart])
			} else {
				// Emit until end of stream, no end indicator found
				emit(nals[prevStart:])
			}
		}
	}
}

const (
	naluTypeBitmask   = 0x1F
)
