package transcontainer

import "fmt"

type State uint8
const (
	START           State = 0
	STOP	        State = 1
	STREAM			State = 2
)
func (s State) String() string {
	switch s {
	case START:
		return "START"
	case STOP:
		return "STOP"
	case STREAM:
		return "STREAM"
	default:
		return fmt.Sprintf("%d", int(s))
	}
}
