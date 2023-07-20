// Code generated by ifacemaker; DO NOT EDIT.

package webrtc

import (
	"github.com/pion/rtcp"
)

// IPeerConnection ...
type IPeerConnection interface {
	// OnSignalingStateChange sets an event handler which is invoked when the
	// peer connection's signaling state changes
	OnSignalingStateChange(f func(SignalingState))
	// OnDataChannel sets an event handler which is invoked when a data
	// channel message arrives from a remote peer.
	OnDataChannel(f func(*DataChannel))
	// OnNegotiationNeeded sets an event handler which is invoked when
	// a change has occurred which requires session negotiation
	OnNegotiationNeeded(f func())
	// OnICECandidate sets an event handler which is invoked when a new ICE
	// candidate is found.
	// Take note that the handler is gonna be called with a nil pointer when
	// gathering is finished.
	OnICECandidate(f func(*ICECandidate))
	// OnICEGatheringStateChange sets an event handler which is invoked when the
	// ICE candidate gathering state has changed.
	OnICEGatheringStateChange(f func(ICEGathererState))
	// OnTrack sets an event handler which is called when remote track
	// arrives from a remote peer.
	OnTrack(f func(*TrackRemote, *RTPReceiver))
	// OnICEConnectionStateChange sets an event handler which is called
	// when an ICE connection state is changed.
	OnICEConnectionStateChange(f func(ICEConnectionState))
	// OnConnectionStateChange sets an event handler which is called
	// when the PeerConnectionState has changed
	OnConnectionStateChange(f func(PeerConnectionState))
	// SetConfiguration updates the configuration of this PeerConnection object.
	SetConfiguration(configuration Configuration) error
	// GetConfiguration returns a Configuration object representing the current
	// configuration of this PeerConnection object. The returned object is a
	// copy and direct mutation on it will not take affect until SetConfiguration
	// has been called with Configuration passed as its only argument.
	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-getconfiguration
	GetConfiguration() Configuration
	// CreateOffer starts the PeerConnection and generates the localDescription
	// https://w3c.github.io/webrtc-pc/#dom-rtcpeerconnection-createoffer
	CreateOffer(options *OfferOptions) (SessionDescription, error)
	// CreateAnswer starts the PeerConnection and generates the localDescription
	CreateAnswer(options *AnswerOptions) (SessionDescription, error)
	// SetLocalDescription sets the SessionDescription of the local peer
	SetLocalDescription(desc SessionDescription) error
	// LocalDescription returns PendingLocalDescription if it is not null and
	// otherwise it returns CurrentLocalDescription. This property is used to
	// determine if SetLocalDescription has already been called.
	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-localdescription
	LocalDescription() *SessionDescription
	// SetRemoteDescription sets the SessionDescription of the remote peer
	// nolint: gocyclo
	SetRemoteDescription(desc SessionDescription) error
	// RemoteDescription returns pendingRemoteDescription if it is not null and
	// otherwise it returns currentRemoteDescription. This property is used to
	// determine if setRemoteDescription has already been called.
	// https://www.w3.org/TR/webrtc/#dom-rtcpeerconnection-remotedescription
	RemoteDescription() *SessionDescription
	// AddICECandidate accepts an ICE candidate string and adds it
	// to the existing set of candidates.
	AddICECandidate(candidate ICECandidateInit) error
	// ICEConnectionState returns the ICE connection state of the
	// PeerConnection instance.
	ICEConnectionState() ICEConnectionState
	// GetSenders returns the RTPSender that are currently attached to this PeerConnection
	GetSenders() (result []*RTPSender)
	// GetReceivers returns the RTPReceivers that are currently attached to this PeerConnection
	GetReceivers() (receivers []*RTPReceiver)
	// GetTransceivers returns the RtpTransceiver that are currently attached to this PeerConnection
	GetTransceivers() []*RTPTransceiver
	// AddTrack adds a Track to the PeerConnection
	AddTrack(track TrackLocal) (*RTPSender, error)
	// RemoveTrack removes a Track from the PeerConnection
	RemoveTrack(sender *RTPSender) (err error)
	// AddTransceiverFromKind Create a new RtpTransceiver and adds it to the set of transceivers.
	AddTransceiverFromKind(kind RTPCodecType, init ...RTPTransceiverInit) (t *RTPTransceiver, err error)
	// AddTransceiverFromTrack Create a new RtpTransceiver(SendRecv or SendOnly) and add it to the set of transceivers.
	AddTransceiverFromTrack(track TrackLocal, init ...RTPTransceiverInit) (t *RTPTransceiver, err error)
	// CreateDataChannel creates a new DataChannel object with the given label
	// and optional DataChannelInit used to configure properties of the
	// underlying channel such as data reliability.
	CreateDataChannel(label string, options *DataChannelInit) (*DataChannel, error)
	CreateIDataChannel(label string, options *DataChannelInit) (IDataChannel, error)
	// SetIdentityProvider is used to configure an identity provider to generate identity assertions
	SetIdentityProvider(provider string) error
	// WriteRTCP sends a user provided RTCP packet to the connected peer. If no peer is connected the
	// packet is discarded. It also runs any configured interceptors.
	WriteRTCP(pkts []rtcp.Packet) error
	// Close ends the PeerConnection
	Close() error
	// CurrentLocalDescription represents the local description that was
	// successfully negotiated the last time the PeerConnection transitioned
	// into the stable state plus any local candidates that have been generated
	// by the ICEAgent since the offer or answer was created.
	CurrentLocalDescription() *SessionDescription
	// PendingLocalDescription represents a local description that is in the
	// process of being negotiated plus any local candidates that have been
	// generated by the ICEAgent since the offer or answer was created. If the
	// PeerConnection is in the stable state, the value is null.
	PendingLocalDescription() *SessionDescription
	// CurrentRemoteDescription represents the last remote description that was
	// successfully negotiated the last time the PeerConnection transitioned
	// into the stable state plus any remote candidates that have been supplied
	// via AddICECandidate() since the offer or answer was created.
	CurrentRemoteDescription() *SessionDescription
	// PendingRemoteDescription represents a remote description that is in the
	// process of being negotiated, complete with any remote candidates that
	// have been supplied via AddICECandidate() since the offer or answer was
	// created. If the PeerConnection is in the stable state, the value is
	// null.
	PendingRemoteDescription() *SessionDescription
	// SignalingState attribute returns the signaling state of the
	// PeerConnection instance.
	SignalingState() SignalingState
	// ICEGatheringState attribute returns the ICE gathering state of the
	// PeerConnection instance.
	ICEGatheringState() ICEGatheringState
	// ConnectionState attribute returns the connection state of the
	// PeerConnection instance.
	ConnectionState() PeerConnectionState
	// GetStats return data providing statistics about the overall connection
	GetStats() StatsReport
	// SCTP returns the SCTPTransport for this PeerConnection
	//
	// The SCTP transport over which SCTP data is sent and received. If SCTP has not been negotiated, the value is nil.
	// https://www.w3.org/TR/webrtc/#attributes-15
	SCTP() *SCTPTransport
}
