package media

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProtectionMetadata_MarshalMeta(t *testing.T) {

	meta := &Meta{
		EncScheme:   2,
		IsProtected: true,
		Offset:      2,
		Kid:         [16]byte{42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
		Iv:          [16]byte{42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
	}

	bytes := meta.Marshal(nil, nil)
	require.Equal(t, []byte{40, 2, 42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 0}, bytes)

	_meta := &Meta{}
	require.Nil(t, _meta.Unmarshal(bytes), "failed to unmarshal meta")

	verifyMeta(t, meta, _meta)
}

func TestProtectionMetadata_MarshalSubsamples(t *testing.T) {

	subsamples := &Subsamples{
		SubsampleCount:  3,
		SubsamplesTotal: 210,
		Subsamples: []Subsample{
			{
				ClearDataByteSize:     10,
				ProtectedDataByteSize: 20,
			}, {
				ClearDataByteSize:     30,
				ProtectedDataByteSize: 40,
			}, {
				ClearDataByteSize:     50,
				ProtectedDataByteSize: 60,
			},
		},
	}

	bytes := subsamples.Marshal()
	require.Equal(t, []byte{3, 0, 10, 0, 0, 0, 20, 0, 30, 0, 0, 0, 40, 0, 50, 0, 0, 0, 60}, bytes)

	_subsamples := &Subsamples{}
	require.Nil(t, _subsamples.Unmarshal(bytes), "failed to unmarshal subsamples")

	verifySubsamples(t, subsamples, _subsamples)
}

func TestProtectionMetadata_MarshalPattern(t *testing.T) {

	pattern := &Pattern{
		CryptByteBlock: 2,
		SkipByteBlock:  5,
	}

	bytes := pattern.Marshal()
	require.Equal(t, []byte{37}, bytes)

	_pattern := &Pattern{}
	require.Nil(t, _pattern.Unmarshal(bytes), "failed to unmarshal Pattern")

	verifyPattern(t, pattern, _pattern)
}

func TestProtectionMetadata_MarshalProtectionMeta_Empty(t *testing.T) {

	protectionMeta := &ProtectionMeta{}

	extensions := protectionMeta.Marshal()

	require.Empty(t, extensions, "extensions length should be 0")
}

func TestProtectionMetadata_MarshalProtectionMeta_Simple(t *testing.T) {

	protectionMeta := &ProtectionMeta{
		Meta: &Meta{
			EncScheme:   2,
			IsProtected: true,
			Offset:      2,
			Kid:         [16]byte{42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
			Iv:          [16]byte{42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
		},
	}

	extensions := protectionMeta.Marshal()

	require.Equal(t, 1, len(extensions), "extensions length should be 1")
	require.NotNil(t, extensions[7], "extensions id should be 7")
	require.Equal(t, []byte{40, 2, 42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 0}, extensions[7])

	_meta := &Meta{}
	require.Nil(t, _meta.Unmarshal(extensions[7]), "failed to unmarshal meta from extensions")

	verifyMeta(t, protectionMeta.Meta, _meta)
}

func TestProtectionMetadata_MarshalProtectionMeta_WithEmptySubsamples(t *testing.T) {

	protectionMeta := &ProtectionMeta{
		Meta: &Meta{
			EncScheme:   2,
			IsProtected: true,
			Offset:      2,
			Kid:         [16]byte{42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
			Iv:          [16]byte{42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
		},
		Subsamples: &Subsamples{
			SubsampleCount: 0,
		},
	}

	extensions := protectionMeta.Marshal()

	require.Equal(t, 1, len(extensions), "extensions length should be 1")
	require.NotNil(t, extensions[7], "extensions id should be 7")
	require.Equal(t, []byte{40, 2, 42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 0}, extensions[7])

	_meta := &Meta{}
	require.Nil(t, _meta.Unmarshal(extensions[7]), "failed to unmarshal meta from extensions")

	verifyMeta(t, protectionMeta.Meta, _meta)
}

func TestProtectionMetadata_MarshalProtectionMeta_WithSubsamples(t *testing.T) {

	protectionMeta := &ProtectionMeta{
		Meta: &Meta{
			EncScheme:   2,
			IsProtected: true,
			Offset:      2,
			Kid:         [16]byte{42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
			Iv:          [16]byte{42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
		},
		Subsamples: &Subsamples{
			SubsampleCount:  3,
			SubsamplesTotal: 210,
			Subsamples: []Subsample{
				{
					ClearDataByteSize:     10,
					ProtectedDataByteSize: 20,
				}, {
					ClearDataByteSize:     30,
					ProtectedDataByteSize: 40,
				}, {
					ClearDataByteSize:     50,
					ProtectedDataByteSize: 60,
				},
			},
		},
	}

	extensions := protectionMeta.Marshal()

	require.Equal(t, 2, len(extensions), "extensions length should be 2")
	require.NotNil(t, extensions[7], "extensions id for meta should be 7")
	require.NotNil(t, extensions[8], "extensions id for subsamples should be 8")

	require.Equal(t, []byte{44, 2, 42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 0}, extensions[7])
	require.Equal(t, []byte{3, 0, 10, 0, 0, 0, 20, 0, 30, 0, 0, 0, 40, 0, 50, 0, 0, 0, 60}, extensions[8])

	_meta := &Meta{}
	require.Nil(t, _meta.Unmarshal(extensions[7]), "failed to unmarshal meta from extensions")

	verifyMeta(t, protectionMeta.Meta, _meta)

	_subsamples := &Subsamples{}
	require.Nil(t, _subsamples.Unmarshal(extensions[8]), "failed to unmarshal subsamples from extensions")

	verifySubsamples(t, protectionMeta.Subsamples, _subsamples)
}

func TestProtectionMetadata_MarshalProtectionMeta_WithEmptyPattern(t *testing.T) {

	protectionMeta := &ProtectionMeta{
		Meta: &Meta{
			EncScheme:   2,
			IsProtected: true,
			Offset:      2,
			Kid:         [16]byte{42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
			Iv:          [16]byte{42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
		},
		Pattern: &Pattern{
			CryptByteBlock: 0,
			SkipByteBlock:  0,
		},
	}

	extensions := protectionMeta.Marshal()

	require.Equal(t, 1, len(extensions), "extensions length should be 1")
	require.NotNil(t, extensions[7], "extensions id for meta should be 7")
	require.Equal(t, []byte{40, 2, 42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 0}, extensions[7])

	_meta := &Meta{}
	require.Nil(t, _meta.Unmarshal(extensions[7]), "failed to unmarshal meta from extensions")

	verifyMeta(t, protectionMeta.Meta, _meta)
}

func TestProtectionMetadata_MarshalProtectionMeta_WithPattern(t *testing.T) {

	protectionMeta := &ProtectionMeta{
		Meta: &Meta{
			EncScheme:   2,
			IsProtected: true,
			Offset:      2,
			Kid:         [16]byte{42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
			Iv:          [16]byte{42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
		},
		Pattern: &Pattern{
			CryptByteBlock: 2,
			SkipByteBlock:  5,
		},
	}

	extensions := protectionMeta.Marshal()

	require.Equal(t, 2, len(extensions), "extensions length should be 2")
	require.NotNil(t, extensions[7], "extensions id for meta should be 7")
	require.NotNil(t, extensions[9], "extensions id for Pattern should be 9")

	require.Equal(t, []byte{42, 2, 42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 0}, extensions[7])
	require.Equal(t, []byte{37}, extensions[9])

	_meta := &Meta{}
	require.Nil(t, _meta.Unmarshal(extensions[7]), "failed to unmarshal meta from extensions")

	verifyMeta(t, protectionMeta.Meta, _meta)

	_pattern := &Pattern{}
	require.Nil(t, _pattern.Unmarshal(extensions[9]), "failed to unmarshal Pattern from extensions")

	verifyPattern(t, protectionMeta.Pattern, _pattern)
}

func TestProtectionMetadata_MarshalProtectionMeta_WithSubsamplesAndPattern(t *testing.T) {

	protectionMeta := &ProtectionMeta{
		Meta: &Meta{
			EncScheme:   2,
			IsProtected: true,
			Offset:      2,
			Kid:         [16]byte{42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
			Iv:          [16]byte{42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42},
		},
		Subsamples: &Subsamples{
			SubsampleCount:  3,
			SubsamplesTotal: 210,
			Subsamples: []Subsample{
				{
					ClearDataByteSize:     10,
					ProtectedDataByteSize: 20,
				}, {
					ClearDataByteSize:     30,
					ProtectedDataByteSize: 40,
				}, {
					ClearDataByteSize:     50,
					ProtectedDataByteSize: 60,
				},
			},
		},
		Pattern: &Pattern{
			CryptByteBlock: 2,
			SkipByteBlock:  5,
		},
	}

	extensions := protectionMeta.Marshal()

	require.Equal(t, 3, len(extensions), "extensions length should be 3")
	require.NotNil(t, extensions[7], "extensions id for meta should be 7")
	require.NotNil(t, extensions[8], "extensions id for subsamples should be 8")
	require.NotNil(t, extensions[9], "extensions id for Pattern should be 9")

	require.Equal(t, []byte{46, 2, 42, 42, 107, 105, 100, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 42, 42, 105, 118, 69, 120, 97, 109, 112, 108, 101, 58, 41, 42, 42, 0}, extensions[7])
	require.Equal(t, []byte{3, 0, 10, 0, 0, 0, 20, 0, 30, 0, 0, 0, 40, 0, 50, 0, 0, 0, 60}, extensions[8])
	require.Equal(t, []byte{37}, extensions[9])

	_meta := &Meta{}
	require.Nil(t, _meta.Unmarshal(extensions[7]), "failed to unmarshal meta from extensions")

	verifyMeta(t, protectionMeta.Meta, _meta)

	_subsamples := &Subsamples{}
	require.Nil(t, _subsamples.Unmarshal(extensions[8]), "failed to unmarshal subsamples from extensions")

	verifySubsamples(t, protectionMeta.Subsamples, _subsamples)

	_pattern := &Pattern{}
	require.Nil(t, _pattern.Unmarshal(extensions[9]), "failed to unmarshal Pattern from extensions")

	verifyPattern(t, protectionMeta.Pattern, _pattern)
}

func verifyMeta(t *testing.T, meta *Meta, _meta *Meta) {
	require.Equal(t, meta.EncScheme, _meta.EncScheme, "encryption scheme should be set to 2")
	require.True(t, _meta.IsProtected, "IsProtected should be set to true")
	require.Equal(t, meta.Offset, _meta.Offset, "Offset should be set to 2")

	require.Equal(t, meta.Kid, _meta.Kid, "unexpected Kid")
	require.Equal(t, meta.Iv, _meta.Iv, "unexpected Iv")
}

func verifySubsamples(t *testing.T, subsamples *Subsamples, _subsamples *Subsamples) {
	require.Equal(t, subsamples.SubsampleCount, _subsamples.SubsampleCount, "there should be 3 subsamples")
	require.Equal(t, subsamples.SubsamplesTotal, _subsamples.SubsamplesTotal, "SubsamplesTotal should be 90")

	for i := 0; i < 3; i++ {
		require.Equal(t, subsamples.Subsamples[i].ClearDataByteSize, _subsamples.Subsamples[i].ClearDataByteSize, fmt.Sprintf("unexpected inClear for subsample #%d", i))
		require.Equal(t, subsamples.Subsamples[i].ProtectedDataByteSize, _subsamples.Subsamples[i].ProtectedDataByteSize, fmt.Sprintf("unexpected inEncrypted for subsample #%d", i))
	}
}

func verifyPattern(t *testing.T, Pattern *Pattern, _pattern *Pattern) {
	require.Equal(t, Pattern.CryptByteBlock, _pattern.CryptByteBlock, "unexpected CryptByteBlock")
	require.Equal(t, Pattern.SkipByteBlock, _pattern.SkipByteBlock, "unexpected SkipByteBlock")
}
