package gst

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gst.h"

*/
import "C"
import (
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/utils"
	log "github.com/sirupsen/logrus"
	"sync"
	"unsafe"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/webrtc/rtpbuffer"
	"github.com/pion/webrtc/v3/pkg/media"
)

func init() {
	go C.gstreamer_send_start_mainloop()
}

// Pipeline is a wrapper for a GStreamer Pipeline
type Pipeline struct {
	Pipeline   *C.GstElement
	audioTrack *webrtc.Track
	videoTrack *webrtc.Track
	Type string
	OnSampleHandler	func(sample media.Sample, streamType utils.StreamType, sampleType utils.SampleType)
}

var jitter = &rtpbuffer.Jitter{}
var uiPacketizationMode = 1
var abrPacketizationMode = 1

var GLOBAL_STATE="ui"

var pipeline = &Pipeline{}
var pipelinesLock sync.Mutex

func SetJitter(j *rtpbuffer.Jitter){
	jitter = j
}


func ResetGlobalState(){
	GLOBAL_STATE = "ui"
}

// CreatePipeline creates a GStreamer Pipeline
func CreatePipeline(pipelineStr string, audioTrack, videoTrack *webrtc.Track, pipelineType string, onSampleHandler func(media.Sample, utils.StreamType, utils.SampleType)) *Pipeline {
	// from file
	//pipelineStr := fmt.Sprintf("filesrc location=\"%s\" ! decodebin name=demux ! queue ! x264enc bframes=0 speed-preset=veryfast key-int-max=60 ! video/x-h264,stream-format=byte-stream ! appsink name=video demux. ! queue ! audioconvert ! audioresample ! opusenc ! appsink name=audio", containerPath)

	// hls reendcode with framerate
	//pipelineStr := "souphttpsrc location=http://devimages.apple.com/iphone/samples/bipbop/gear4/prog_index.m3u8 ! hlsdemux ! decodebin name=demux ! queue ! videorate ! video/x-raw,framerate=25/1 ! x264enc bframes=0 speed-preset=veryfast key-int-max=60  ! video/x-h264,stream-format=byte-stream ! appsink name=video"

	// hls no reencocde
	//pipelineStr := fmt.Sprintf("souphttpsrc location=\"%s\" ! hlsdemux ! decodebin3 name=demux caps=video/x-h264,stream-format=byte-stream ! appsink name=video demux. ! queue ! audioconvert ! audioresample ! opusenc ! appsink name=audio", containerPath)

	pipelineStrUnsafe := C.CString(pipelineStr)
	defer C.free(unsafe.Pointer(pipelineStrUnsafe))

	pipelinesLock.Lock()
	defer pipelinesLock.Unlock()
	pipeline = &Pipeline{
		Pipeline:   C.gstreamer_send_create_pipeline(pipelineStrUnsafe),
		audioTrack: audioTrack,
		videoTrack: videoTrack,
		Type: pipelineType,
		OnSampleHandler: onSampleHandler,
	}
	return pipeline
}

// Start starts the GStreamer Pipeline
func (p *Pipeline) Start() {
	// This will signal to goHandlePipelineBuffer
	// and provide a method for cancelling sends.

	isAbr := C.int(1)
	if (p.Type == "ui") {
		isAbr = C.int(0)
	}

	C.gstreamer_send_start_pipeline(p.Pipeline, isAbr)
}

// Play sets the pipeline to PLAYING
func (p *Pipeline) Play() {
	C.gstreamer_send_play_pipeline(p.Pipeline)
}

// Pause sets the pipeline to PAUSED
func (p *Pipeline) Pause() {
	C.gstreamer_send_pause_pipeline(p.Pipeline)
}

// Stop sets the pipeline to PAUSED
func (p *Pipeline) Stop() {
	C.gstreamer_send_stop_pipeline(p.Pipeline)
}

// SeekToTime seeks on the pipeline
func (p *Pipeline) SeekToTime(seekPos int64) {
	C.gstreamer_send_seek(p.Pipeline, C.int64_t(seekPos))
}

const (
	videoClockRate = 90000
	audioClockRate = 48000
	maxDonValue = ^uint16(0)
)

var don = uint16(0)
var donSize = 2
//export goHandlePipelineBuffer
func goHandlePipelineBuffer(buffer unsafe.Pointer, bufferLen C.int, duration C.int, isVideo C.int, isAbr C.int) {

	//if isAbr == 0 && GLOBAL_STATE == "switch_to_ui" {
	//	if isVideo == 1 && isIframe(buffer, bufferLen) {
	//		GLOBAL_STATE = "ui"
	//	}
	//} else if isAbr == 1 && GLOBAL_STATE == "switch_to_abr" {
	//	if isVideo == 1 && isIframe(buffer, bufferLen) {
	//		GLOBAL_STATE = "abr"
	//	}
	//}
	//if isAbr == 1 && (GLOBAL_STATE == "ui" || GLOBAL_STATE == "switch_to_abr") || isAbr == 0 && (GLOBAL_STATE == "abr"|| GLOBAL_STATE == "switch_to_ui") {
	//	return
	//}

	//var track *webrtc.Track
	var samples uint32

	if isVideo == 1 {
		samples = videoClockRate / uint32(25) //uint32(videoClockRate * (float32(duration) / 1000000000))
		//track = pipeline.videoTrack
	} else {
		samples = uint32(audioClockRate * (float32(duration) / 1000000000))
		//track = pipeline.audioTrack
	}

	log.WithFields(
		log.Fields{
			"component": "gst",
			"isVideo": isVideo,
			"isAbr": isAbr,
			"GLOBAL_STATE": GLOBAL_STATE,
		}).Trace("writing sample")


	pipeline.OnSampleHandler(media.Sample{Data: C.GoBytes(buffer, bufferLen), Samples: samples}, utils.ABR, utils.SampleType(int(isVideo)))

	//if isVideo == 1{
	//	var frame = C.GoBytes(buffer, bufferLen)
	//
	//	// in interleaved mode we calculate the DON and send it down to the packetizer
	//	if packetizationMode == 2 {
	//		frameWithDon := make([]byte, donSize + len(frame))
	//		binary.BigEndian.PutUint16(frameWithDon[:2], don)
	//		copy(frameWithDon[donSize:], frame)
	//		//if err := jitter.WriteSample(media.Sample{Data: frameWithDon, Samples: samples}); err != nil && err != io.ErrClosedPipe {
	//		//	panic(err)
	//		//}
	//		pipeline.OnSampleHandler(media.Sample{Data: frameWithDon, Samples: samples}, int(isVideo))
	//
	//		don = (don+1) % maxDonValue
	//	} else{
	//		pipeline.OnSampleHandler(media.Sample{Data: frame, Samples: samples}, int(isVideo))
	//	}
	//
	//} else {
	//	pipeline.OnSampleHandler(media.Sample{Data: C.GoBytes(buffer, bufferLen), Samples: samples}, int(isVideo))
	//}

	C.free(buffer)
}
