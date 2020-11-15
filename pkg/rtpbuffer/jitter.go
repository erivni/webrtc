package rtpbuffer

import (
	"fmt"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"sync"
	"time"
)

type Jitter struct {
	buffer map[uint16]*rtp.Packet
	peerConnection *webrtc.PeerConnection
	track *webrtc.Track
}

var segmentStart = uint16(0)
var segmentEnd = uint16(0)
var currentSN = uint16(0)
var mapSync = sync.RWMutex{}

func NewJitter(pc *webrtc.PeerConnection, t *webrtc.Track) *Jitter {
	return &Jitter{buffer: make(map[uint16]*rtp.Packet), peerConnection: pc, track: t}
}

func (j *Jitter) StartRTCP(){
	go func(){
		nackCount := 0
		senders := j.peerConnection.GetSenders()
		if len(senders) < 1{
			fmt.Println("found no senders")
		}
		sender := senders[0]
		for{
			packets, _ := sender.ReadRTCP()
			//fmt.Println("number of packets read: ", len(packets))

			for _, packet := range packets{
				//fmt.Println("packet type: ", reflect.TypeOf(packet))
				switch packet := packet.(type) {
				case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
					fmt.Println("got pli")
				case *rtcp.ReceiverEstimatedMaximumBitrate:
					fmt.Println("got pli")
				case *rtcp.TransportLayerNack:
					nack := packet

					for _, nack := range nack.Nacks{
						nackCount++
						//fmt.Println("nackCount: ", nackCount, ". got nack for packet ", nack.PacketID)
						mapSync.Lock()
						j.track.WriteRTP(j.buffer[nack.PacketID])
						mapSync.Unlock()
					}
				default:
				}
			}
		}
	}()

	// clean old RTP packets
	go func(){
		for range time.NewTicker(5 * time.Second).C {
			fmt.Println("buffer size: ", len(j.buffer))
			if segmentEnd == 0{
				segmentEnd = currentSN
			}else{
				// clear previous segment
				for i := segmentStart; i<=segmentEnd; i++{
					if _, ok := j.buffer[i]; ok {
						mapSync.Lock()
						delete(j.buffer, i)
						mapSync.Unlock()
					}
				}
				segmentStart = segmentEnd + 1
				segmentEnd = currentSN
			}
		}
	}()
}

func (j *Jitter) WriteSample(s media.Sample) error {
	packets := j.track.Packetizer().Packetize(s.Data, s.Samples)
	//fmt.Println("packets in frame: ", len(packets))

	// prefer sending packets over cleaning packets
	for _, p := range packets {
		if segmentStart == 0{
			segmentStart = p.Header.SequenceNumber
		}
		currentSN = p.Header.SequenceNumber
		mapSync.Lock()
		j.buffer[p.Header.SequenceNumber] = p
		mapSync.Unlock()
		err := j.track.WriteRTP(p)
		if err != nil {
			return err
		}
	}


	return nil
}

