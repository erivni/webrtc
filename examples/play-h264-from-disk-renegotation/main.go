package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/pion/randutil"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/h264reader"
)

var peerConnection *webrtc.PeerConnection //nolint

// doSignaling exchanges all state of the local PeerConnection and is called
// every time a video is added or removed
func doSignaling(w http.ResponseWriter, r *http.Request) {
	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		panic(err)
	}

	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	response, err := json.Marshal(*peerConnection.LocalDescription())
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(response); err != nil {
		panic(err)
	}
}

// Add a single video track
func createPeerConnection(w http.ResponseWriter, r *http.Request) {
	if peerConnection.ConnectionState() != webrtc.PeerConnectionStateNew {
		panic(fmt.Sprintf("createPeerConnection called in non-new state (%s)", peerConnection.ConnectionState()))
	}

	doSignaling(w, r)
	fmt.Println("PeerConnection has been created")
}

// Add a single video track
func addVideo(w http.ResponseWriter, r *http.Request) {
	videoTrack, err := peerConnection.NewTrack(
		webrtc.DefaultPayloadTypeH264,
		randutil.NewMathRandomGenerator().Uint32(),
		fmt.Sprintf("video-%d", randutil.NewMathRandomGenerator().Uint32()),
		fmt.Sprintf("video-%d", randutil.NewMathRandomGenerator().Uint32()),
	)
	if err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTrack(videoTrack); err != nil {
		panic(err)
	}

	go writeVideoToTrack(videoTrack)
	doSignaling(w, r)
	fmt.Println("Video track has been added")
}

// Remove a single sender
func removeVideo(w http.ResponseWriter, r *http.Request) {
	if senders := peerConnection.GetSenders(); len(senders) != 0 {
		if err := peerConnection.RemoveTrack(senders[0]); err != nil {
			panic(err)
		}
	}

	doSignaling(w, r)
	fmt.Println("Video track has been removed")
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	var err error
	if peerConnection, err = webrtc.NewPeerConnection(webrtc.Configuration{}); err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/createPeerConnection", createPeerConnection)
	http.HandleFunc("/addVideo", addVideo)
	http.HandleFunc("/removeVideo", removeVideo)

	fmt.Println("Open http://localhost:8080 to access this demo")
	panic(http.ListenAndServe(":8080", nil))
}

// Read a video file from disk and write it to a webrtc.Track
// When the video has been completely read this exits without error
func writeVideoToTrack(t *webrtc.Track) {

	go func() {
		file, h264Err := os.Open("output.h264")
		if h264Err != nil {
			panic(h264Err)
		}

		h264, h264Err := h264reader.NewReader(file)
		if h264Err != nil {
			panic(h264Err)
		}

		spsAndPpsCache := []byte{}
		for {
			nal, h264Err := h264.NextNAL()
			if h264Err == io.EOF {
				fmt.Printf("All video frames parsed and sent")
				os.Exit(0)
			}
			if h264Err != nil {
				panic(h264Err)
			}

			time.Sleep(time.Millisecond * 33)

			nal.Data = append([]byte{0x00, 0x00, 0x00, 0x01}, nal.Data...)

			if nal.UnitType == h264reader.NalUnitTypeSPS || nal.UnitType == h264reader.NalUnitTypePPS {
				spsAndPpsCache = append(spsAndPpsCache, nal.Data...)
				continue
			} else if nal.UnitType == h264reader.NalUnitTypeCodedSliceIdr {
				nal.Data = append(spsAndPpsCache, nal.Data...)
				spsAndPpsCache = []byte{}
			}

			if h264Err = t.WriteSample(media.Sample{Data: nal.Data, Samples: 90000}); h264Err != nil {
				panic(h264Err)
			}
		}
	}()
}
