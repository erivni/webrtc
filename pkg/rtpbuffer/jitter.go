package rtpbuffer

import (
	"fmt"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type Jitter struct {
	buffer map[uint16]*rtp.Packet
	peerConnection *webrtc.PeerConnection
	track *webrtc.Track
	segmentStart uint16
	segmentEnd uint16
	currentSN uint16
	mapSync sync.RWMutex
	stop  bool
}

func NewJitter(pc *webrtc.PeerConnection, t *webrtc.Track) *Jitter {
	jitter := &Jitter{
		buffer: make(map[uint16]*rtp.Packet),
		peerConnection: pc,
		track: t,
		segmentStart: uint16(0),
		segmentEnd: uint16(0),
		currentSN: uint16(0),
		mapSync: sync.RWMutex{},
		stop: false}
	jitter.startRTCP()
	return  jitter
}

func (j *Jitter) Close(){
	j.stop = true
	for k := range j.buffer{
		delete(j.buffer, k)
	}
	j.buffer = nil
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

			for _, packet := range packets{
				/*
				log.WithFields(
					log.Fields{
						"component": "jitter",
						"segmentStart": segmentStart,
						"segmentEnd": segmentEnd,
						"size": len(j.buffer),
					}).Info("rtcp: got ", reflect.TypeOf(packet), " packet")

				 */

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
						j.mapSync.Lock()
						j.track.WriteRTP(j.buffer[nack.PacketID])
						j.mapSync.Unlock()
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

			if j.segmentEnd == 0{
				j.segmentEnd = j.currentSN
				log.WithFields(
					log.Fields{
						"component": "jitter",
						"segmentStart": j.segmentStart,
						"segmentEnd": j.segmentEnd,
						"size": len(j.buffer),
					}).Info("cleanup: first cleanup. setting segmentEnd")
			}else{
				log.WithFields(
					log.Fields{
						"component": "jitter",
						"segmentStart": j.segmentStart,
						"segmentEnd": j.segmentEnd,
						"size": len(j.buffer),
					}).Info("cleanup. about to delete ", j.segmentEnd - j.segmentStart, " entries")

				// clear previous segment
				for i := j.segmentStart; i<=j.segmentEnd; i++{
					if _, ok := j.buffer[i]; ok {
						j.mapSync.Lock()
						delete(j.buffer, i)
						j.mapSync.Unlock()
					}
				}
				j.segmentStart = j.segmentEnd + 1
				j.segmentEnd = j.currentSN
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
		if j.segmentStart == 0{
			j.segmentStart = p.Header.SequenceNumber
			log.WithFields(
				log.Fields{
					"component": "jitter",
					"segmentStart": j.segmentStart,
					"segmentEnd": j.segmentEnd,
					"size": len(j.buffer),
				}).Info("first packet for connection")
		}
		j.currentSN = p.Header.SequenceNumber
		j.mapSync.Lock()
		j.buffer[p.Header.SequenceNumber] = p
		j.mapSync.Unlock()
		err := j.track.WriteRTP(p)
		if err != nil {
			return err
		}
	}


	return nil
}

