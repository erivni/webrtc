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
	stop  bool
}

var segmentStart = uint16(0)
var segmentEnd = uint16(0)
var currentSN = uint16(0)
var mapSync = sync.RWMutex{}


func NewJitter(pc *webrtc.PeerConnection, t *webrtc.Track) *Jitter {
	jitter := &Jitter{buffer: make(map[uint16]*rtp.Packet), peerConnection: pc, track: t, stop: false}
	jitter.startRTCP()
	return  jitter
}

func (j *Jitter) Close(){
	j.stop = true
	for k := range j.buffer{
		delete(j.buffer, k)
	}
}

func (j *Jitter) startRTCP(){
	go func(){
		nackCount := 0
		senders := j.peerConnection.GetSenders()
		if len(senders) < 1{
			fmt.Println("found no senders")
		}
		sender := senders[0]
		for{
			if j.stop == true{
				return
			}
			packets, _ := sender.ReadRTCP()
			//fmt.Println("number of packets read: ", len(packets))

			for _, packet := range packets{
				//fmt.Println("packet type: ", reflect.TypeOf(packet))
				switch packet := packet.(type) {
				case *rtcp.PictureLossIndication:
					//fmt.Println("got pli")
				case *rtcp.FullIntraRequest:
					//fmt.Println("got fir")
				case *rtcp.ReceiverEstimatedMaximumBitrate:
					//fmt.Println("got remb")
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
			if j.stop == true{
				return
			}
			//fmt.Println("buffer size: ", len(j.buffer))
			if segmentEnd == 0{
				segmentEnd = currentSN
			}else{
				//fmt.Println("segments ", segmentStart, " ", segmentEnd, " removing ", segmentEnd - segmentStart)
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
				//fmt.Println("after assigning. start ", segmentStart, " end: ", segmentEnd, ". size after delete ", len(j.buffer))
			}
		}
	}()
}

func (j *Jitter) WriteSample(s media.Sample) error {
	if j.stop == true{
		return nil
	}
	packets := j.track.Packetizer().Packetize(s.Data, s.Samples)
	//fmt.Println("packets in frame: ", len(packets))

	// prefer sending packets over cleaning packets
	for _, p := range packets {
		if segmentStart == 0{
			segmentStart = p.Header.SequenceNumber
		}
		currentSN = p.Header.SequenceNumber
		//fmt.Println("current sn ", currentSN)
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

