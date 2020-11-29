package rtpbuffer

import (
	"fmt"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	log "github.com/sirupsen/logrus"
	"net"
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
	nackCount int
	pliCount int
	firCount int
	rembCount int
	receiverTotalLost uint32
	forwardRtp bool
	udpCon net.Conn
	stop  bool
}

func NewJitter(pc *webrtc.PeerConnection, t *webrtc.Track, forwardRtp bool) *Jitter {
	jitter := &Jitter{
		buffer: make(map[uint16]*rtp.Packet),
		peerConnection: pc,
		track: t,
		segmentStart: uint16(0),
		segmentEnd: uint16(0),
		currentSN: uint16(0),
		nackCount: int(0),
		pliCount: int(0),
		firCount: int(0),
		rembCount: int(0),
		receiverTotalLost: uint32(0),
		mapSync: sync.RWMutex{},
		forwardRtp: forwardRtp,
		stop: false}
	if forwardRtp == true{
		jitter.initUdp()
	}

	return  jitter
}

func (j *Jitter) initUdp (){
	conn, _ := net.Dial("udp", "127.0.0.1:4002")
	j.udpCon = conn
}

func (j *Jitter) Close(){
	if j == nil{
		return
	}
	j.stop = true
	for k := range j.buffer{
		delete(j.buffer, k)
	}
	j.buffer = nil

	if closeErr := j.udpCon.Close(); closeErr != nil {
		log.Error("error closing udp connection")
	}
}

func (j *Jitter) StartRTCP(){
	go func(){
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
					j.pliCount++
				case *rtcp.FullIntraRequest:
					j.firCount++
				case *rtcp.ReceiverEstimatedMaximumBitrate:
					j.rembCount++
				case *rtcp.TransportLayerNack:
					nack := packet
					for _, nack := range nack.Nacks{
						j.nackCount++
						//fmt.Println("nackCount: ", nackCount, ". got nack for packet ", nack.PacketID)
						j.mapSync.Lock()
						packet, ok := j.buffer[nack.PacketID]
						j.mapSync.Unlock()
						if ok == false{
							log.Warn("did not find packet with SN ", nack.PacketID, " in jitter")
						}else{
							j.track.WriteRTP(packet)
						}
					}
				case *rtcp.ReceiverReport:
					if len(packet.Reports) > 0{
						j.receiverTotalLost = packet.Reports[0].TotalLost
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

	go func(){
		for range time.NewTicker(30 * time.Second).C {

			if j.stop == true{
				return
			}

			log.WithFields(
				log.Fields{
					"component": "jitter",
					"nackCount": j.nackCount,
					"pliCount": j.pliCount,
					"friCount": j.firCount,
					"rembCount": j.rembCount,
					"receiverTotalLost": j.receiverTotalLost,
					"segmentStart": j.segmentStart,
					"segmentEnd": j.segmentEnd,
					"size": len(j.buffer),
				}).Info("jitter report")


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
		if err := j.track.WriteRTP(p); err != nil{
			log.WithFields(
				log.Fields{
					"component": "jitter",
					"rtp SN": p.SequenceNumber,
				}).Error("error sending rtp packet")
		}

		if j.forwardRtp{
			pBytes, err := p.Marshal()
			_, err = j.udpCon.Write(pBytes)
			if opError, ok := err.(*net.OpError); ok && opError.Err.Error() == "write: connection refused" {
				continue
			}
		}
	}

	return nil
}

type udpConn struct {
	conn *net.UDPConn
	port int
}
