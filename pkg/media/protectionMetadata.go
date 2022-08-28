// Provides structs for protection metadata
package media

import (
	"encoding/binary"
	"errors"
)

type Meta struct {
	EncScheme   uint
	IsProtected bool
	Iv          [16]byte
	Kid         [16]byte
}

type Subsample struct {
	ClearDataByteSize     uint16
	ProtectedDataByteSize uint32
}

type Subsamples struct {
	SubsampleCount  uint8
	SubsamplesTotal uint64
	Subsamples      []Subsample
}

type Pattern struct {
	CryptByteBlock uint8
	SkipByteBlock  uint8
}

type ProtectionMeta struct {
	Meta       *Meta
	Subsamples *Subsamples
	Pattern    *Pattern
}

func (meta *Meta) Marshal(subsamples *Subsamples, pattern *Pattern) []byte {
	// 0-8 in bigEndian (in reverse)

	var metaByte byte = 0

	metaByte |= (byte(meta.EncScheme) & 0x0F) << 4 // bits(0-3)

	if meta.IsProtected { // bit(4)
		metaByte |= 1 << 3
	}
	if subsamples != nil && subsamples.SubsampleCount > 0 { // bit(5)
		metaByte |= 1 << 2
	}
	if pattern != nil && (pattern.CryptByteBlock > 0 || pattern.SkipByteBlock > 0) { // bit(6)
		metaByte |= 1 << 1
	}

	// bit(7) reserved

	metaBytes := []byte{metaByte}

	metaBytes = append(metaBytes, meta.Kid[:]...)
	metaBytes = append(metaBytes, meta.Iv[:]...)

	return metaBytes
}

func (meta *Meta) Unmarshal(bytes []byte) error {

	/*
	 *  0                   1
	 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |Scheme |E|S|P|R| Offset	       |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |Kid (#1)                       |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |...                            |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |Kid (#8)                       |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |Iv (#1)                        |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |..                             |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |Iv (#8)                        |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	if len(bytes) != 33 {
		return errors.New("unexpected meta bytes length")
	}
	if bytes[0]&1 != byte(0) {
		return errors.New("reserved bit should be set to 0")
	}

	meta.EncScheme = uint(bytes[0] >> 4)
	meta.IsProtected = ((bytes[0] >> 3) & 1) == byte(1)

	copy(meta.Kid[:], bytes[1:17])
	copy(meta.Iv[:], bytes[17:32])

	return nil
}

func (subsamples *Subsamples) Marshal() []byte {
	sampleCountOffset := 1 // 1byte reserved for SubsampleCount
	sizeOfClear := 2       // 16bits
	sizeOfProtected := 4   // 32bits
	sizeOfPair := sizeOfClear + sizeOfProtected

	subsampleBytes := make([]byte, sampleCountOffset+(sizeOfPair)*int(subsamples.SubsampleCount))
	subsampleBytes[0] = subsamples.SubsampleCount

	for i, subsample := range subsamples.Subsamples {
		binary.BigEndian.PutUint16(subsampleBytes[sampleCountOffset+(i*sizeOfPair):], subsample.ClearDataByteSize)
		binary.BigEndian.PutUint32(subsampleBytes[sampleCountOffset+(i*sizeOfPair+sizeOfClear):], subsample.ProtectedDataByteSize)
	}

	return subsampleBytes
}

func (subsamples *Subsamples) Unmarshal(bytes []byte) error {
	/*
	 *  0                   1                   2                   3
	 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |subsampleCnt   | inClear                       | inEncrypted   |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * | ...                                           | inClear       |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * | ...          | inEncrypted                                    |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	if len(bytes) <= 1 {
		return nil
	}

	sizeOfClear := 2
	sizeOfEncrypted := 4
	sizeOfPair := sizeOfClear + sizeOfEncrypted

	if (len(bytes)-1)%sizeOfPair != 0 {
		return errors.New("unexpected subsamples bytes length")
	}

	subsamples.SubsampleCount = bytes[0]
	subsamples.Subsamples = make([]Subsample, subsamples.SubsampleCount)

	subsamplesBytes := bytes[1:] // skip SubsampleCount position

	for i := 0; i < int(subsamples.SubsampleCount); i++ {
		subsamples.Subsamples[i] = Subsample{
			ClearDataByteSize:     binary.BigEndian.Uint16(subsamplesBytes[i*sizeOfPair:]),
			ProtectedDataByteSize: binary.BigEndian.Uint32(subsamplesBytes[i*sizeOfPair+sizeOfClear:]),
		}
		subsamples.SubsamplesTotal += uint64(subsamples.Subsamples[i].ClearDataByteSize) + uint64(subsamples.Subsamples[i].ProtectedDataByteSize)
	}
	return nil
}

func (pattern *Pattern) Marshal() []byte {
	var patternBytes byte = 0
	patternBytes |= pattern.CryptByteBlock << 4 // bits(0-3)
	patternBytes |= pattern.SkipByteBlock       // bits(4-7)

	return []byte{patternBytes}
}

func (pattern *Pattern) Unmarshal(bytes []byte) error {
	/*
	 *  0
	 *  0 1 2 3 4 5 6 7
	 * +-+-+-+-+-+-+-+-+
	 * |CryptBB| SkipBB|
	 * +-+-+-+-+-+-+-+-+
	 */

	if len(bytes) != 1 {
		return errors.New("unexpected pattern bytes length")
	}

	pattern.CryptByteBlock = bytes[0] >> 4 // bits(0-3)
	pattern.SkipByteBlock = bytes[0] & 0xF // bits(4-7)

	return nil
}
