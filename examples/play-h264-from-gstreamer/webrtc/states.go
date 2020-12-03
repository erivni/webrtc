package webrtc

import "fmt"

type State uint8
const (
	IDLE         State = 0
	INITIATE     State = 1
	CONNECTED    State = 2
	DISCONNECTED State = 3
	FAILED       State = 4
)

func (s State) String() string {
	switch s {
	case IDLE:
		return "IDLE"
	case INITIATE:
		return "INITIATE"
	case CONNECTED:
		return "CONNECTED"
	case DISCONNECTED:
		return "DISCONNECTED"
	case FAILED:
		return "FAILED"
	default:
		return fmt.Sprintf("%d", int(s))
	}
}
