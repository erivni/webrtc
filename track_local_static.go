//go:build !js
// +build !js

package webrtc

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/encryption"
	"github.com/pion/webrtc/v3/internal/util"
	"github.com/pion/webrtc/v3/pkg/media"

	log "github.com/sirupsen/logrus"
)

// trackBinding is a single bind for a Track
// Bind can be called multiple times, this stores the
// result for a single bind call so that it can be used when writing
type trackBinding struct {
	id          string
	ssrc        SSRC
	payloadType PayloadType
	writeStream TrackLocalWriter
}

// TrackLocalStaticRTP  is a TrackLocal that has a pre-set codec and accepts RTP Packets.
// If you wish to send a media.Sample use TrackLocalStaticSample
type TrackLocalStaticRTP struct {
	mu           sync.RWMutex
	bindings     []trackBinding
	codec        RTPCodecCapability
	id, streamID string

	numberOfPackets uint64
	sizeBytes       uint64
}

// NewTrackLocalStaticRTP returns a TrackLocalStaticRTP.
func NewTrackLocalStaticRTP(c RTPCodecCapability, id, streamID string) (*TrackLocalStaticRTP, error) {
	return &TrackLocalStaticRTP{
		codec:    c,
		bindings: []trackBinding{},
		id:       id,
		streamID: streamID,
	}, nil
}

// Bind is called by the PeerConnection after negotiation is complete
// This asserts that the code requested is supported by the remote peer.
// If so it setups all the state (SSRC and PayloadType) to have a call
func (s *TrackLocalStaticRTP) Bind(t TrackLocalContext) (RTPCodecParameters, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parameters := RTPCodecParameters{RTPCodecCapability: s.codec}
	if codec, matchType := codecParametersFuzzySearch(parameters, t.CodecParameters()); matchType != codecMatchNone {
		s.bindings = append(s.bindings, trackBinding{
			ssrc:        t.SSRC(),
			payloadType: codec.PayloadType,
			writeStream: t.WriteStream(),
			id:          t.ID(),
		})
		return codec, nil
	}

	return RTPCodecParameters{}, ErrUnsupportedCodec
}

// Unbind implements the teardown logic when the track is no longer needed. This happens
// because a track has been stopped.
func (s *TrackLocalStaticRTP) Unbind(t TrackLocalContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.bindings {
		if s.bindings[i].id == t.ID() {
			s.bindings[i] = s.bindings[len(s.bindings)-1]
			s.bindings = s.bindings[:len(s.bindings)-1]
			return nil
		}
	}

	return ErrUnbindFailed
}

// ID is the unique identifier for this Track. This should be unique for the
// stream, but doesn't have to globally unique. A common example would be 'audio' or 'video'
// and StreamID would be 'desktop' or 'webcam'
func (s *TrackLocalStaticRTP) ID() string { return s.id }

// StreamID is the group this track belongs too. This must be unique
func (s *TrackLocalStaticRTP) StreamID() string { return s.streamID }

// Kind controls if this TrackLocal is audio or video
func (s *TrackLocalStaticRTP) Kind() RTPCodecType {
	switch {
	case strings.HasPrefix(s.codec.MimeType, "audio/"):
		return RTPCodecTypeAudio
	case strings.HasPrefix(s.codec.MimeType, "video/"):
		return RTPCodecTypeVideo
	default:
		return RTPCodecType(0)
	}
}

// Codec gets the Codec of the track
func (s *TrackLocalStaticRTP) Codec() RTPCodecCapability {
	return s.codec
}

func (s *TrackLocalStaticRTP) GetStats() (uint64, uint64) {
	return s.numberOfPackets, s.sizeBytes
}

// packetPool is a pool of packets used by WriteRTP and Write below
// nolint:gochecknoglobals
var rtpPacketPool = sync.Pool{
	New: func() interface{} {
		return &rtp.Packet{}
	},
}

// WriteRTP writes a RTP Packet to the TrackLocalStaticRTP
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them
func (s *TrackLocalStaticRTP) WriteRTP(p *rtp.Packet) error {
	ipacket := rtpPacketPool.Get()
	packet := ipacket.(*rtp.Packet)
	defer func() {
		*packet = rtp.Packet{}
		rtpPacketPool.Put(ipacket)
	}()
	*packet = *p

	s.numberOfPackets++
	s.sizeBytes += 15 + uint64(packet.Length)

	return s.writeRTP(packet)
}

// writeRTP is like WriteRTP, except that it may modify the packet p
func (s *TrackLocalStaticRTP) writeRTP(p *rtp.Packet) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	writeErrs := []error{}

	for _, b := range s.bindings {
		p.Header.SSRC = uint32(b.ssrc)
		p.Header.PayloadType = uint8(b.payloadType)
		log.WithFields(
			log.Fields{
				"type":           "INTENSIVE",
				"subcomponent":   "webrtc",
				"ssrc":           p.Header.SSRC,
				"timestamp":      p.Timestamp,
				"sequenceNumber": p.SequenceNumber,
				"hasExtension":   p.Extension,
				"extensions":     fmt.Sprintf("%v", p.Extensions),
			}).Trace("outgoing rtp..")
		if _, err := b.writeStream.WriteRTP(&p.Header, p.Payload); err != nil {
			writeErrs = append(writeErrs, err)
		}
	}

	return util.FlattenErrs(writeErrs)
}

// Write writes a RTP Packet as a buffer to the TrackLocalStaticRTP
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them
func (s *TrackLocalStaticRTP) Write(b []byte) (n int, err error) {
	ipacket := rtpPacketPool.Get()
	packet := ipacket.(*rtp.Packet)
	defer func() {
		*packet = rtp.Packet{}
		rtpPacketPool.Put(ipacket)
	}()

	if err = packet.Unmarshal(b); err != nil {
		return 0, err
	}

	return len(b), s.writeRTP(packet)
}

// TrackLocalStaticSample is a TrackLocal that has a pre-set codec and accepts Samples.
// If you wish to send a RTP Packet use TrackLocalStaticRTP
type TrackLocalStaticSample struct {
	Packetizer           rtp.Packetizer
	sequencer            rtp.Sequencer
	RtpTrack             *TrackLocalStaticRTP
	hyperscaleEncryption bool
	encryption           *encryption.Encryption
	ClockRate            float64
}

// NewTrackLocalStaticSample returns a TrackLocalStaticSample
func NewTrackLocalStaticSample(c RTPCodecCapability, id, streamID string) (*TrackLocalStaticSample, error) {
	rtpTrack, err := NewTrackLocalStaticRTP(c, id, streamID)
	if err != nil {
		return nil, err
	}

	track := &TrackLocalStaticSample{
		RtpTrack: rtpTrack,
	}

	track.hyperscaleEncryption = os.Getenv("HYPERSCALE_RTP_ENCRYPTION_ACTIVE") == "true"
	if track.hyperscaleEncryption {
		track.encryption = encryption.NewEncryption()
	}
	return track, nil
}

func (s *TrackLocalStaticSample) SetHyperscaleEncryption(active bool) {
	s.hyperscaleEncryption = active
}

// ID is the unique identifier for this Track. This should be unique for the
// stream, but doesn't have to globally unique. A common example would be 'audio' or 'video'
// and StreamID would be 'desktop' or 'webcam'
func (s *TrackLocalStaticSample) ID() string { return s.RtpTrack.ID() }

// StreamID is the group this track belongs too. This must be unique
func (s *TrackLocalStaticSample) StreamID() string { return s.RtpTrack.StreamID() }

// Kind controls if this TrackLocal is audio or video
func (s *TrackLocalStaticSample) Kind() RTPCodecType { return s.RtpTrack.Kind() }

// Codec gets the Codec of the track
func (s *TrackLocalStaticSample) Codec() RTPCodecCapability {
	return s.RtpTrack.Codec()
}

func (s *TrackLocalStaticSample) GetStats() (uint64, uint64) {
	return s.RtpTrack.GetStats()
}

// Bind is called by the PeerConnection after negotiation is complete
// This asserts that the code requested is supported by the remote peer.
// If so it setups all the state (SSRC and PayloadType) to have a call
func (s *TrackLocalStaticSample) Bind(t TrackLocalContext) (RTPCodecParameters, error) {
	codec, err := s.RtpTrack.Bind(t)
	if err != nil {
		return codec, err
	}

	s.RtpTrack.mu.Lock()
	defer s.RtpTrack.mu.Unlock()

	// We only need one Packetizer
	if s.Packetizer != nil {
		return codec, nil
	}

	payloader, err := payloaderForCodec(codec.RTPCodecCapability)
	if err != nil {
		return codec, err
	}

	s.sequencer = rtp.NewRandomSequencer()
	s.Packetizer = rtp.NewInterleavedPacketizer(
		getRtpOutboundMtu(),
		0, // Value is handled when writing
		0, // Value is handled when writing
		payloader,
		s.sequencer,
		codec.ClockRate,
	)
	s.ClockRate = float64(codec.RTPCodecCapability.ClockRate)
	return codec, nil
}

// Unbind implements the teardown logic when the track is no longer needed. This happens
// because a track has been stopped.
func (s *TrackLocalStaticSample) Unbind(t TrackLocalContext) error {
	return s.RtpTrack.Unbind(t)
}

// WriteSample writes a Sample to the TrackLocalStaticSample
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them
func (s *TrackLocalStaticSample) WriteSample(sample media.Sample, onRtpPacket func(*rtp.Packet)) error {
	s.RtpTrack.mu.RLock()
	p := s.Packetizer
	clockRate := s.ClockRate
	s.RtpTrack.mu.RUnlock()

	if p == nil {
		return nil
	}

	// skip packets by the number of previously dropped packets
	for i := uint16(0); i < sample.PrevDroppedPackets; i++ {
		s.sequencer.NextSequenceNumber()
	}

	samples := uint32(sample.Duration.Seconds() * clockRate)
	if sample.PrevDroppedPackets > 0 {
		p.(rtp.Packetizer).SkipSamples(samples * uint32(sample.PrevDroppedPackets))
	}

	payloadDataIdx := -1
	var packets []*rtp.Packet

	if s.hyperscaleEncryption {
		packets, payloadDataIdx = p.(rtp.Packetizer).PacketizeAndDetectData(sample.Data, uint32(samples))
	} else {
		packets = p.(rtp.Packetizer).Packetize(sample.Data, uint32(samples))
	}

	err := addExtensions(sample, packets, s.hyperscaleEncryption, s.encryption, payloadDataIdx)

	log.WithFields(
		log.Fields{
			"subcomponent":   "webrtc",
			"type":           "INTENSIVE",
			"isIframe":       sample.IsIFrame,
			"payloadDataIdx": payloadDataIdx,
		}).Trace("write sample: dataIndex ", payloadDataIdx)

	if err != nil {
		log.WithFields(
			log.Fields{
				"subcomponent": "webrtc",
				"type":         "INTENSIVE",
				"err":          err.Error(),
				"hasExtension": packets[0].Extension,
				"extensions":   fmt.Sprintf("%v", packets[0].Extensions),
			}).Error("encountered an error when adding extension")
	}

	writeErrs := []error{}
	maxPacketsBurst, packetsSpreadDelay := getPacketsSpreadConfig(sample)
	packetsSentWithoutDelay := 0
	for _, p := range packets {
		if maxPacketsBurst > 0 && packetsSpreadDelay > 0 && packetsSentWithoutDelay >= maxPacketsBurst {
			packetsSentWithoutDelay = 0
			time.Sleep(time.Duration(packetsSpreadDelay) * time.Millisecond)
		}
		if err := s.RtpTrack.WriteRTP(p); err != nil {
			writeErrs = append(writeErrs, err)
		} else {
			packetsSentWithoutDelay++
		}
		if onRtpPacket != nil {
			onRtpPacket(p)
		}
	}

	return util.FlattenErrs(writeErrs)
}

// WriteSample writes a Sample to the TrackLocalStaticSample
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them
func (s *TrackLocalStaticSample) WriteInterleavedSample(sample media.Sample, onRtpPacket func(*rtp.Packet)) error {
	s.RtpTrack.mu.RLock()
	p := s.Packetizer
	clockRate := s.ClockRate
	s.RtpTrack.mu.RUnlock()

	if p == nil {
		return nil
	}

	samples := sample.Duration.Seconds() * clockRate

	payloadDataIdx := -1
	var packets []*rtp.Packet

	if s.hyperscaleEncryption {
		packets, payloadDataIdx = p.(rtp.Packetizer).PacketizeInterleavedAndDetectData(sample.Data, uint32(samples))
	} else {
		packets = p.(rtp.Packetizer).PacketizeInterleaved(sample.Data, uint32(samples))
	}

	err := addExtensions(sample, packets, s.hyperscaleEncryption, s.encryption, payloadDataIdx)

	log.WithFields(
		log.Fields{
			"subcomponent":   "webrtc",
			"type":           "INTENSIVE",
			"isIframe":       sample.IsIFrame,
			"payloadDataIdx": payloadDataIdx,
		}).Trace("write interleaved sample: dataIndex ", payloadDataIdx)

	if err != nil {
		log.WithFields(
			log.Fields{
				"subcomponent": "webrtc",
				"type":         "INTENSIVE",
				"err":          err.Error(),
				"hasExtension": packets[0].Extension,
				"extensions":   fmt.Sprintf("%v", packets[0].Extensions),
			}).Error("encountered an error when adding extension")
	}

	writeErrs := []error{}
	maxPacketsBurst, packetsSpreadDelay := getPacketsSpreadConfig(sample)
	packetsSentWithoutDelay := 0
	for _, p := range packets {
		if maxPacketsBurst > 0 && packetsSpreadDelay > 0 && packetsSentWithoutDelay >= maxPacketsBurst {
			packetsSentWithoutDelay = 0
			time.Sleep(time.Duration(packetsSpreadDelay) * time.Millisecond)
		}
		if err := s.RtpTrack.WriteRTP(p); err != nil {
			writeErrs = append(writeErrs, err)
		} else {
			packetsSentWithoutDelay++
		}
		if onRtpPacket != nil {
			onRtpPacket(p)
		}
	}

	return util.FlattenErrs(writeErrs)
}

func addExtensions(sample media.Sample, packets []*rtp.Packet, hyperscaleEncryption bool, encryption *encryption.Encryption, payloadDataIdx int) error {
	var sampleAttr byte = 0
	sampleAttr |= 1 << getExtensionVal("HYPERSCALE_RTP_EXTENSION_FIRST_PACKET_ATTR_POS", 0)
	if sample.IsIFrame {
		sampleAttr |= 1 << getExtensionVal("HYPERSCALE_RTP_EXTENSION_IFRAME_ATTR_POS", 1)
	}
	if sample.IsSpsPps {
		sampleAttr |= 1 << getExtensionVal("HYPERSCALE_RTP_EXTENSION_SPS_PPS_ATTR_POS", 2)
	}
	if sample.IsAbr {
		sampleAttr |= 1 << getExtensionVal("HYPERSCALE_RTP_EXTENSION_ABR_ATTR_POS", 3)
	}

	var shouldEncryptFirstPacket, resultWillNotChangeFirstPacket = false, false
	encPosition := getExtensionVal("HYPERSCALE_RTP_EXTENSION_ENCRYPTION_ATTR_POS", 5)

	if hyperscaleEncryption {
		shouldEncryptFirstPacket, resultWillNotChangeFirstPacket = encryption.ShouldEncrypt(sample, 0, payloadDataIdx)
		if !shouldEncryptFirstPacket {
			// set the 'skip encryption' bit
			sampleAttr |= 1 << encPosition
		}
	}

	var extensionErrs []error
	attributesExtId := getExtensionVal("HYPERSCALE_RTP_EXTENSION_SAMPLE_ATTR_ID", 5)

	if len(packets) > 0 {

		// add protectionMetadata extensions. ids [7-9]
		// --------------------------------------------------------
		// !! must be added first to configure extension profile !!
		// --------------------------------------------------------
		if sample.ProtectionMeta != nil {
			if sample.ProtectionMeta.Meta != nil {
				id := getExtensionVal("HYPERSCALE_RTP_EXTENSION_PROTECTION_META_ID", 7)
				extensionErrs = append(extensionErrs, packets[0].SetExtension(id, sample.ProtectionMeta.Meta.Marshal(sample.ProtectionMeta.Subsamples, sample.ProtectionMeta.Pattern)))
			}
			if sample.ProtectionMeta.Subsamples != nil && sample.ProtectionMeta.Subsamples.SubsampleCount > 0 {
				id := getExtensionVal("HYPERSCALE_RTP_EXTENSION_PROTECTION_SUBSAMPLES_ID", 8)
				extensionErrs = append(extensionErrs, packets[0].SetExtension(id, sample.ProtectionMeta.Subsamples.Marshal()))
			}
			if sample.ProtectionMeta.Pattern != nil && sample.ProtectionMeta.Pattern.SkipByteBlock != 0 && sample.ProtectionMeta.Pattern.CryptByteBlock != 0 {
				id := getExtensionVal("HYPERSCALE_RTP_EXTENSION_PROTECTION_PATTERN_ID", 9)
				extensionErrs = append(extensionErrs, packets[0].SetExtension(id, sample.ProtectionMeta.Pattern.Marshal()))
			}
		}

		// add sample predefined extensions. e.g: id [3]
		extensionErrs = append(extensionErrs, packets[0].SetExtensions(sample.Extensions))

		if sample.WithHyperscaleExtensions {
			extensionErrs = append(extensionErrs, packets[0].SetExtension(attributesExtId, []byte{sampleAttr}))

			// add sample dts extension. id [6]
			if isDtsExtensionEnabled() {
				id := getExtensionVal("HYPERSCALE_RTP_EXTENSION_SAMPLE_DTS_ID", 6)
				dtsBytes := make([]byte, 8)
				binary.BigEndian.PutUint64(dtsBytes, sample.Dts)
				extensionErrs = append(extensionErrs, packets[0].SetExtension(id, dtsBytes))
			}

			// add frame_seq extension. id [4]
			id := getExtensionVal("HYPERSCALE_RTP_EXTENSION_DON_ID", 4)
			donBytes := make([]byte, 2)
			binary.BigEndian.PutUint16(donBytes, sample.Don)
			extensionErrs = append(extensionErrs, packets[0].SetExtension(id, donBytes))

		}
	}

	// since default is to encrypt, if first packet returned 'encrypt' and result will not change for next packets
	// no need to check the rest of the packets
	stopChecking := shouldEncryptFirstPacket && resultWillNotChangeFirstPacket

	if hyperscaleEncryption && !stopChecking {
		// now check whether the rest of the packets need to be encrypted
		for i := 1; i < len(packets); i++ {
			sampleAttr = 0
			shouldEncryptPacket, resultWillNotChangeRemainingPackets := encryption.ShouldEncrypt(sample, i, payloadDataIdx)

			// set the 'skip encryption' bit
			if !shouldEncryptPacket {
				sampleAttr |= 1 << encPosition
				extensionErrs = append(extensionErrs, packets[i].SetExtension(attributesExtId, []byte{sampleAttr}))
			}

			// since default is to encrypt, i.e. we set the bit to 'skip encryption',
			// if this packet should encrypt and rest of the packets have the same result, no need to look further
			if shouldEncryptPacket && resultWillNotChangeRemainingPackets {
				break
			}
		}
	}

	return util.FlattenErrs(extensionErrs)
}

func getExtensionVal(envVariable string, defaultValue uint8) uint8 {
	envValue := os.Getenv(envVariable)
	if envValue != "" {
		parsed, err := strconv.ParseUint(envValue, 10, 8)
		if err == nil {
			return uint8(parsed)
		}
	}
	return defaultValue
}

func isDtsExtensionEnabled() bool {
	envValue := os.Getenv("HYPERSCALE_RTP_EXTENSION_SAMPLE_DTS_ENABLED")
	return envValue == "true"
}

func getRtpOutboundMtu() uint16 {
	rtpOutboundMTUEnv := os.Getenv("HYPERSCALE_WEBRTC_RTP_OUTBOUND_MTU")
	if rtpOutboundMTUEnv != "" {
		parsed, err := strconv.ParseUint(rtpOutboundMTUEnv, 10, 16)
		if err == nil {
			return uint16(parsed)
		}
	}
	return rtpOutboundMTU
}

func getPacketsSpreadConfig(sample media.Sample) (int, int) {
	packetsSpreadDelay := 0
	maxPacketsBurst := 0
	if sample.IsAbr {
		packetsSpreadDelay, _ = strconv.Atoi(os.Getenv("HYPERSCALE_ABR_SPREAD_PACKETS_DELAY_MS")) // 0 is returned on error, in which case feature will be ignored later on
		maxPacketsBurst, _ = strconv.Atoi(os.Getenv("HYPERSCALE_ABR_MAX_PACKET_BURST"))           // 0 is returned on error, in which case feature will be ignored later on
	} else {
		packetsSpreadDelay, _ = strconv.Atoi(os.Getenv("HYPERSCALE_UI_SPREAD_PACKETS_DELAY_MS")) // 0 is returned on error, in which case feature will be ignored later on
		maxPacketsBurst, _ = strconv.Atoi(os.Getenv("HYPERSCALE_UI_MAX_PACKET_BURST"))           // 0 is returned on error, in which case feature will be ignored later on
	}
	return maxPacketsBurst, packetsSpreadDelay
}
