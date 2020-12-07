package transcontainer

import "C"
import (
	"encoding/binary"
	"fmt"
	"github.com/pion/rtcp"
	pion "github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/gst"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/utils"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/webrtc"
	"github.com/pion/webrtc/v3/pkg/media"
	log "github.com/sirupsen/logrus"
	"strings"
)

const (
	maxDonValue = ^uint16(0)
	donSize = 2
)

type State uint8
const (
	IDLE			State = 0
	STARTED         State = 1
	STOPPED         State = 2
)
func (s State) String() string {
	switch s {
	case IDLE:
		return "IDLE"
	case STARTED:
		return "STARTED"
	case STOPPED:
		return "STOPPED"
	default:
		return fmt.Sprintf("%d", int(s))
	}
}


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

	// TODO: can we have a connectionId for login
	//connectionId string

	uiConnection *webrtc.WebRTCClient
	clientConnection *webrtc.WebRTCServer

	abrPlayer *gst.Pipeline

	don 	  uint16
	OnStateChangeHandler func(State)
	OnStreamingStateChangeHandler func(StreamingState)
}

func NewTranscontainer(uiConnection *webrtc.WebRTCClient, clientConnection *webrtc.WebRTCServer, abrPlayer *gst.Pipeline, onStateChangeHandler func(State), OnStreamingStateChangeHandler func(StreamingState)) *Transcontainer {
	return &Transcontainer{
		State:                IDLE,
		StreamingState:       UI,
		don:				  0,
		uiConnection:         uiConnection,
		clientConnection:     clientConnection,
		abrPlayer:            abrPlayer,
		OnStateChangeHandler: onStateChangeHandler,
		OnStreamingStateChangeHandler: OnStreamingStateChangeHandler,
	}
}

func (t *Transcontainer) Start() {


	t.changeState(STARTED)
	log.WithFields(
		log.Fields{
			"component": 		"transcontainer",
			"state":			t.State,
			"streamingState":	t.StreamingState,
		}).Info("starting transcontainer")

	t.uiConnection.SetListeners(t.processUiMessage, t.processSample)
	t.clientConnection.SetListeners(t.processClientMessage, t.processRTCP)
	t.abrPlayer.OnSampleHandler = t.processSample

	t.don = 0

	// start reading from ui connection
	t.uiConnection.StartReadingRTPs()
	t.abrPlayer.Start()
}

// ProcessRTCP receives RTCP messages from the client connection and makes decisions based on packet type and state
// e.g. when in UI we can decide to lower bitrate. when in ABR we can decide to lock on a different ABR bitrate
func (t *Transcontainer) processRTCP(packet rtcp.Packet){

	log.WithFields(
		log.Fields{
			"component": 		"transcontainer",
			"state": 			t.State,
			"steamingState": 	t.StreamingState,
		}).Trace("process rtcp...")

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
	log.WithFields(
		log.Fields{
			"component": "transcontainer",
			"state": 			t.State,
			"steamingState": 	t.StreamingState,
			"datachannelMsg": string(msg.Data),
		}).Info("got message from ui.")

	t.processMessage(string(msg.Data))
}

func (t *Transcontainer) processClientMessage(msg pion.DataChannelMessage){
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

func (t *Transcontainer) Stop() {

	log.WithFields(
		log.Fields{
			"component": 		"transcontainer",
			"state":			t.State,
			"streamingState":	t.StreamingState,
		}).Debug("stopping transcontainer...")

	t.changeState(STOPPED)
	t.changeStreamingState(UI)

	t.uiConnection.StopReadingRTP()
	t.abrPlayer.Stop()

}

func (t *Transcontainer) changeState(state State) {
	if state == t.State{
		return
	}
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
