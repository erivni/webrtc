package main

import (
	"context"
	"fmt"
	"github.com/pion/randutil"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/examples/internal/signal"
	"github.com/pion/webrtc/v3/examples/play-h264-from-gstreamer/gst"
)

func main() {
	sdpChan := signal.HTTPSDPServer()

	// Everything below is the Pion WebRTC API, thanks for using it ❤️.
	offer := webrtc.SessionDescription{}
	signal.Decode(<-sdpChan, &offer)

	// We make our own mediaEngine so we can place the sender's codecs in it.  This because we must use the
	// dynamic media type from the sender in our answer. This is not required if we are the offerer
	mediaEngine := webrtc.MediaEngine{}
	if err := mediaEngine.PopulateFromSDP(offer); err != nil {
		panic(err)
	}

	// Create a new RTCPeerConnection
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
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

	containerPath := "./spiderman.mp4";

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

	pipeline := &gst.Pipeline{}
	pipeline = gst.CreatePipeline(containerPath, audioTrack, videoTrack)

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			fmt.Println(signal.Encode(candidate.ToJSON()))
		}
	})

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
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
	fmt.Println(signal.Encode(*peerConnection.LocalDescription()))

	<-iceConnectedCtx.Done()

	pipeline.Start()


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
