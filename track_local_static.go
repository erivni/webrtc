// +build !js

package webrtc

import (
	"strings"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/internal/util"
	"github.com/pion/webrtc/v3/pkg/media"
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
func (s *TrackLocalStaticRTP) Bind(t TrackLocalContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	parameters := RTPCodecParameters{RTPCodecCapability: s.codec}
	if codec, err := codecParametersFuzzySearch(parameters, t.CodecParameters()); err == nil {
		s.bindings = append(s.bindings, trackBinding{
			ssrc:        t.SSRC(),
			payloadType: codec.PayloadType,
			writeStream: t.WriteStream(),
			id:          t.ID(),
		})
		return nil
	}

	return ErrUnsupportedCodec
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

// WriteRTP writes a RTP Packet to the TrackLocalStaticRTP
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them
func (s *TrackLocalStaticRTP) WriteRTP(p *rtp.Packet) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	writeErrs := []error{}
	for _, b := range s.bindings {
		p.Header.SSRC = uint32(b.ssrc)
		p.Header.PayloadType = uint8(b.payloadType)
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
	packet := &rtp.Packet{}
	if err = packet.Unmarshal(b); err != nil {
		return 0, err
	}

	return len(b), s.WriteRTP(packet)
}

// TrackLocalStaticSample is a TrackLocal that has a pre-set codec and accepts Samples.
// If you wish to send a RTP Packet use TrackLocalStaticRTP
type TrackLocalStaticSample struct {
	Packetizer rtp.Packetizer
	RtpTrack   *TrackLocalStaticRTP
	ClockRate  float64
}

// NewTrackLocalStaticSample returns a TrackLocalStaticSample
func NewTrackLocalStaticSample(c RTPCodecCapability, id, streamID string) (*TrackLocalStaticSample, error) {
	rtpTrack, err := NewTrackLocalStaticRTP(c, id, streamID)
	if err != nil {
		return nil, err
	}

	return &TrackLocalStaticSample{
		RtpTrack: rtpTrack,
	}, nil
}

// ID is the unique identifier for this Track. This should be unique for the
// stream, but doesn't have to globally unique. A common example would be 'audio' or 'video'
// and StreamID would be 'desktop' or 'webcam'
func (s *TrackLocalStaticSample) ID() string { return s.RtpTrack.ID() }

// StreamID is the group this track belongs too. This must be unique
func (s *TrackLocalStaticSample) StreamID() string { return s.RtpTrack.StreamID() }

// Kind controls if this TrackLocal is audio or video
func (s *TrackLocalStaticSample) Kind() RTPCodecType { return s.RtpTrack.Kind() }

// Bind is called by the PeerConnection after negotiation is complete
// This asserts that the code requested is supported by the remote peer.
// If so it setups all the state (SSRC and PayloadType) to have a call
func (s *TrackLocalStaticSample) Bind(t TrackLocalContext) error {
	if err := s.RtpTrack.Bind(t); err != nil {
		return err
	}

	s.RtpTrack.mu.Lock()
	defer s.RtpTrack.mu.Unlock()

	// We only need one packetizer
	if s.Packetizer != nil {
		return nil
	}

	parameters := RTPCodecParameters{RTPCodecCapability: s.RtpTrack.codec}
	codec, err := codecParametersFuzzySearch(parameters, t.CodecParameters())
	if err != nil {
		return err
	}

	payloader, err := payloaderForCodec(s.RtpTrack.codec)
	if err != nil {
		return err
	}

	s.Packetizer = rtp.NewPacketizer(
		rtpOutboundMTU,
		0, // Value is handled when writing
		0, // Value is handled when writing
		payloader,
		rtp.NewRandomSequencer(),
		codec.ClockRate,
	)
	s.ClockRate = float64(codec.RTPCodecCapability.ClockRate)
	return nil
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
func (s *TrackLocalStaticSample) WriteSample(sample media.Sample) error {
	s.RtpTrack.mu.RLock()
	p := s.Packetizer
	clockRate := s.ClockRate
	s.RtpTrack.mu.RUnlock()

	if p == nil {
		return nil
	}

	samples := sample.Duration.Seconds() * clockRate
	packets := p.(rtp.Packetizer).Packetize(sample.Data, uint32(samples))

	writeErrs := []error{}
	for _, p := range packets {
		if err := s.RtpTrack.WriteRTP(p); err != nil {
			writeErrs = append(writeErrs, err)
		}
	}

	return util.FlattenErrs(writeErrs)
}
