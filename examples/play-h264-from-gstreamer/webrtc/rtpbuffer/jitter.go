package rtpbuffer

import (
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
		mapSync: sync.RWMutex{},
		forwardRtp: forwardRtp,
		stop: false}
	if forwardRtp == true{
		jitter.initUdp()
	}

	jitter.startMaintenance()

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

	if j.udpCon != nil {
		if closeErr := j.udpCon.Close(); closeErr != nil {
			log.Error("error closing udp connection")
		}
	}
}

func (j *Jitter) HandleRTCP(packet rtcp.Packet){
	switch packet := packet.(type) {

	// jitter only handles hacks
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
	default:
	}
}

func (j *Jitter) startMaintenance(){
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


}

func (j *Jitter) WriteSample(s media.Sample) error {
	if j.stop == true{
		return nil
	}
	packets := j.track.Packetizer().Packetize(s.Data, s.Samples)
	//fmt.Println("packets in frame: ", len(packets))

	// prefer sending packets over cleaning packets
	for _, p := range packets {
		j.WriteRTP(p)
	}

	return nil
}

func (j *Jitter) WriteRTP(p *rtp.Packet) error {
	if j.stop == true{
		return nil
	}
	// prefer sending packets over cleaning packets

	//p.Header.SequenceNumber = j.sequenceNumber

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
		}
	}

	//j.sequenceNumber += (j.sequenceNumber + 1) % ^(uint16(0))

	return nil
}
