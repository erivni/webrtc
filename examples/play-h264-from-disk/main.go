package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pion/randutil"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/examples/internal/signal"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/h264reader"
	"github.com/pion/webrtc/v3/pkg/media/oggreader"
)

const (
	audioFileName = "output.ogg"
	videoFileName = "output.h264"
)

func main() {
	// Assert that we have an audio or video file
	_, err := os.Stat(videoFileName)
	haveVideoFile := !os.IsNotExist(err)

	_, err = os.Stat(audioFileName)
	haveAudioFile := !os.IsNotExist(err)

	if !haveAudioFile && !haveVideoFile {
		//panic("Could not find `" + audioFileName + "` or `" + videoFileName + "`")
	}

	sdpChan := signal.HTTPSDPServer()

	// Everything below is the Pion WebRTC API, thanks for using it ❤️.
	offer := webrtc.SessionDescription{}
	signal.Decode(<-sdpChan, &offer)

	// We make our own mediaEngine so we can place the sender's codecs in it.  This because we must use the
	// dynamic media type from the sender in our answer. This is not required if we are the offerer
	mediaEngine := webrtc.MediaEngine{}
	if err = mediaEngine.PopulateFromSDP(offer); err != nil {
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

	if haveVideoFile {
		// Create a video track
		videoTrack, addTrackErr := peerConnection.NewTrack(getPayloadType(mediaEngine, webrtc.RTPCodecTypeVideo, "H264"), randutil.NewMathRandomGenerator().Uint32(), "video", "pion")
		if addTrackErr != nil {
			panic(addTrackErr)
		}
		if _, addTrackErr = peerConnection.AddTrack(videoTrack); addTrackErr != nil {
			panic(addTrackErr)
		}

		go func() {
			var nalStream = make(chan h264reader.Nal)
			fps := uint32(25)

		    <-iceConnectedCtx.Done()

			// to generate a h264 video sample please use sample.sh
			go h264reader.LoadFile(videoFileName, nalStream, fps)

			for {
				nal := <-nalStream
				Samples := uint32(0)
				if nal.UnitType == h264reader.CodedSliceNonIdr || nal.UnitType == h264reader.CodedSliceIdr {
					Samples = 90000 / fps
				}

				// send only Idr, NonIdr, SPS, PPS
				if nal.UnitType == h264reader.CodedSliceNonIdr || nal.UnitType == h264reader.CodedSliceIdr ||
					nal.UnitType == h264reader.SPS || nal.UnitType == h264reader.PPS {
					frame := nal.Data
					// prepend 0x00_00_00_01 prefix if it doesn't not exist
					if !((frame[0] == 0x00 && frame[1] == 0x00 && frame[2] == 0x01) ||
						(frame[0] == 0x00 && frame[1] == 0x00 && frame[2] == 0x00 && frame[3] == 0x01)) {
						frame = append([]byte{0x00, 0x00, 0x00, 0x01}, frame...)
					}
					fmt.Println("nal: ", h264reader.NalUnitTypeStr(nal.UnitType))
					if ivfErr := videoTrack.WriteSample(media.Sample{Data: frame, Samples: Samples}); ivfErr != nil {
						panic(ivfErr)
					}
				}
			}
		}()
	}

	if haveAudioFile {
		// Create a audio track
		audioTrack, addTrackErr := peerConnection.NewTrack(getPayloadType(mediaEngine, webrtc.RTPCodecTypeAudio, "opus"), randutil.NewMathRandomGenerator().Uint32(), "audio", "pion")
		if addTrackErr != nil {
			panic(addTrackErr)
		}
		if _, addTrackErr = peerConnection.AddTrack(audioTrack); addTrackErr != nil {
			panic(addTrackErr)
		}

		go func() {
			// Open a IVF file and start reading using our IVFReader
			file, oggErr := os.Open(audioFileName)
			if oggErr != nil {
				panic(oggErr)
			}

			// Open on oggfile in non-checksum mode.
			ogg, _, oggErr := oggreader.NewWith(file)
			if oggErr != nil {
				panic(oggErr)
			}

			// Wait for connection established
			<-iceConnectedCtx.Done()

			// Keep track of last granule, the difference is the amount of samples in the buffer
			var lastGranule uint64
			for {
				pageData, pageHeader, oggErr := ogg.ParseNextPage()
				if oggErr == io.EOF {
					fmt.Printf("All audio pages parsed and sent")
					os.Exit(0)
				}

				if oggErr != nil {
					panic(oggErr)
				}

				// The amount of samples is the difference between the last and current timestamp
				sampleCount := float64(pageHeader.GranulePosition - lastGranule)
				lastGranule = pageHeader.GranulePosition

				if oggErr = audioTrack.WriteSample(media.Sample{Data: pageData, Samples: uint32(sampleCount)}); oggErr != nil {
					panic(oggErr)
				}

				// Convert seconds to Milliseconds, Sleep doesn't accept floats
				time.Sleep(time.Duration((sampleCount/48000)*1000) * time.Millisecond)
			}
		}()
	}

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

	//ice := webrtc.ICECandidateInit{}
	//signal.Decode(<-iceChan, &ice)
	//
	//if iceErr := peerConnection.AddICECandidate(ice); iceErr != nil {
	//	panic(iceErr)
	//}

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
