package utils

import "fmt"

type StreamType uint8
const (
	UI         StreamType = 0
	ABR     StreamType = 1
)

func (s StreamType) String() string {
	switch s {
	case UI:
		return "UI"
	case ABR:
		return "ABR"
	default:
		return fmt.Sprintf("%d", int(s))
	}
}

type SampleType uint8
const (
	AUDIO   SampleType = 0
	VIDEO   SampleType = 1
)

func (s SampleType) String() string {
	switch s {
	case VIDEO:
		return "VIDEO"
	case AUDIO:
		return "AUDIO"
	default:
		return fmt.Sprintf("%d", int(s))
	}
}
