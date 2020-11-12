package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pion/randutil"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/gst"
	signalling "github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/signallingclient"
	"net/http"
)

func main() {
	signallingClient := signalling.NewSignallingClient("http://34.250.45.79:57778")

	// Everything below is the Pion WebRTC API, thanks for using it ❤️.
	connectionId, err := signallingClient.GetQueue()
	if err != nil {
		panic(err)
	}

	offer, err := signallingClient.GetOffer(connectionId)
	if err != nil {
		panic(err)
	}

	// We make our own mediaEngine so we can place the sender's codecs in it.  This because we must use the
	// dynamic media type from the sender in our answer. This is not required if we are the offerer
	mediaEngine := webrtc.MediaEngine{}
	if err := mediaEngine.PopulateFromSDP(*offer); err != nil {
		panic(err)
	}

	settingEngine := webrtc.SettingEngine{}
	settingEngine.SetEphemeralUDPPortRange(50000, 50002)

	// Create a new RTCPeerConnection
	var api = webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithSettingEngine(settingEngine),
	)
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())

	videoTrack, addTrackErr := peerConnection.NewTrack(getPayloadType(mediaEngine, webrtc.RTPCodecTypeVideo, "H264"), randutil.NewMathRandomGenerator().Uint32(), "video", "pion")
	if addTrackErr != nil {
		panic(addTrackErr)
	}
	if _, addTrackErr = peerConnection.AddTrack(videoTrack); addTrackErr != nil {
		panic(addTrackErr)
	}

	audioTrack, addTrackErr := peerConnection.NewTrack(getPayloadType(mediaEngine, webrtc.RTPCodecTypeAudio, "opus"), randutil.NewMathRandomGenerator().Uint32(), "audio", "pion")
	if addTrackErr != nil {
		panic(addTrackErr)
	}
	if _, addTrackErr = peerConnection.AddTrack(audioTrack); addTrackErr != nil {
		panic(addTrackErr)
	}

	pipelineStr := fmt.Sprintf("souphttpsrc location=http://34.250.45.79:8080/360p_no_bframe_timer_abr.m3u8 ! hlsdemux ! decodebin3 name=demux caps=video/x-h264,stream-format=byte-stream ! appsink name=video demux. ! queue ! audioconvert ! audioresample ! opusenc ! appsink name=audio")

	pipeline := &gst.Pipeline{}
	pipeline = gst.CreatePipeline(pipelineStr, audioTrack, videoTrack, "abr")

	pipelineStrUI := fmt.Sprintf("souphttpsrc location=http://34.250.45.79:8080/360p_no_bframe_timer_ui.m3u8 ! hlsdemux ! decodebin3 name=demux caps=video/x-h264,stream-format=byte-stream ! appsink name=video demux. ! queue ! audioconvert ! audioresample ! opusenc ! appsink name=audio")

	pipelineUI := &gst.Pipeline{}
	pipelineUI = gst.CreatePipeline(pipelineStrUI, audioTrack, videoTrack, "ui")

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			err = signallingClient.SendAnswer(connectionId, *peerConnection.LocalDescription())
			if err != nil {
				panic(err)
			}
		}
	})

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

		// Register channel opening handling
		d.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open.\n", d.Label(), d.ID())
		})

		// Register text message handling
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
			if gst.GLOBAL_STATE != string(msg.Data) {
				gst.GLOBAL_STATE = "switch_to_" + string(msg.Data);
			}
		})
	})

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(*offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	//fmt.Println(signal.Encode(*peerConnection.LocalDescription()))

	<-iceConnectedCtx.Done()

	pipeline.Start()
	pipelineUI.Start()
	//pipelineUI.Pause()

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := peerConnection.GetStats()
		statsStr,_ := json.Marshal(stats)
		w.Header().Set("content-type", "application/json")
		fmt.Fprintf(w, string(statsStr))
	})

	// Block forever
	select {}
}

// Search for Codec PayloadType
//
// Since we are answering we need to match the remote PayloadType
func getPayloadType(m webrtc.MediaEngine, codecType webrtc.RTPCodecType, codecName string) uint8 {
	for _, codec := range m.GetCodecsByKind(codecType) {
		if codec.Name == codecName {
			return codec.PayloadType
		}
	}
	panic(fmt.Sprintf("Remote peer does not support %s", codecName))
}
