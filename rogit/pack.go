package rogit

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

func Unpack(r io.Reader) error {
	br := bufio.NewReader(r)
	var header [12]byte
	var err error
	for i := 0; i < 12; i++ {
		header[i], err = br.ReadByte()
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
	}
	fmt.Println(string(header[:4]))
	fmt.Println("version:", binary.BigEndian.Uint32(header[4:8]))
	fmt.Println("entries:", binary.BigEndian.Uint32(header[8:12]))
	return nil
}

type PackIndex struct {
	ShaSum string
	Offset uint32
}

func PackIDX(file *os.File) ([]PackIndex, error) {
	// Assume version 2
	_, err := file.Seek(8, io.SeekStart)
	if err != nil {
		return nil, err
	}
	// Fanout table
	var fanout [256]uint32
	buf := make([]byte, 4)
	for i := 0; i < 256; i++ {
		_, err = file.Read(buf)
		if err != nil {
			return nil, err
		}
		fanout[i] = binary.BigEndian.Uint32(buf)
	}
	numEntries := fanout[255]
	entries := make([]PackIndex, numEntries)
	buf = make([]byte, 20)
	for i := range entries {
		_, err = file.Read(buf)
		if err != nil {
			return nil, err
		}
		entries[i] = PackIndex{ShaSum: hex.EncodeToString(buf)}
	}
	// Skip CRC for now ;)
	_, err = file.Seek(int64(numEntries*4), io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	// Get offsets
	buf = make([]byte, 4)
	for i := range entries {
		_, err = file.Read(buf)
		if err != nil {
			return nil, err
		}
		entries[i].Offset = binary.BigEndian.Uint32(buf)
	}
	return entries, nil
}

const (
	offsetFanout     = 8
	offsetFanoutSize = 8 + 4*255
	offsetShaListing = 8 + 4*256
)

// SearchPackIDX finds the index of the given shasum in the given pack idx file, if present.
// Returns -1 if not object with the givein shasum could be found.
func SearchPackIDX(idxfile, shasum string) (int64, error) {
	file, err := os.Open(idxfile)
	if err != nil {
		return -1, err
	}
	defer file.Close()
	ss, err := hex.DecodeString(shasum)
	if err != nil {
		return -1, err
	}
	fanoutIndex := int64(ss[0])
	// Assume version 2 (skip first 8 bytes), and look up the shasum in the fanout table
	buf := make([]byte, 4)
	_, err = file.ReadAt(buf, offsetFanout+4*fanoutIndex)
	if err != nil {
		return -1, err
	}
	shaOffset := binary.BigEndian.Uint32(buf)
	var numEntries int64
	_, err = file.ReadAt(buf, offsetFanoutSize)
	if err != nil {
		return -1, err
	}
	numEntries = int64(binary.BigEndian.Uint32(buf))

	buf = make([]byte, 20)
	var i int64
	// Simple O(n)-search, should be replaced with binary search eventually.
	for i = int64(shaOffset - 1); i >= 0; i-- {
		// Skip to the sha listing and offset according to entry currently being checked
		_, err = file.ReadAt(buf, offsetShaListing+20*i)
		if err != nil {
			return -1, err
		}
		if buf[0] != ss[0] {
			// All entries with matching first byte are checked - no match
			return -1, nil
		}
		if hex.EncodeToString(buf) == shasum {
			break
		}
	}
	buf = make([]byte, 4)
	offsetPackfileOffsets := int64(offsetShaListing + 20*numEntries /* (sha listing) */ + 4*numEntries /* (crc) */)
	_, err = file.ReadAt(buf, offsetPackfileOffsets+4*i)
	if err != nil {
		return -1, err
	}
	return int64(binary.BigEndian.Uint32(buf)), nil
}

const (
	moreBit = 0b1000_0000
)

type ObjectType uint8

const (
	OBJ_INVALID ObjectType = 0b000
	OBJ_COMMIT  ObjectType = 0b001
	OBJ_TREE    ObjectType = 0b010
	OBJ_BLOB    ObjectType = 0b011
	OBJ_TAG     ObjectType = 0b100
	// Deltified representations, see https://git-scm.com/docs/pack-format/2.31.0.
	// OBJ_OFS_DELTA refers to the base object by name
	OBJ_OFS_DELTA ObjectType = 0b110
	OBJ_REF_DELTA ObjectType = 0b111
)

func ObjectTypeFromString(otype string) ObjectType {
	switch otype {
	case "commit":
		return OBJ_COMMIT
	case "blob":
		return OBJ_BLOB
	case "tree":
		return OBJ_TREE
	case "tag":
		return OBJ_TAG
	case "ofs-delta":
		return OBJ_OFS_DELTA
	case "ref-delta":
		return OBJ_REF_DELTA
	default:
		return OBJ_INVALID
	}
}

func (ot ObjectType) String() string {
	switch ot {
	case OBJ_COMMIT:
		return "commit"
	case OBJ_BLOB:
		return "blob"
	case OBJ_TREE:
		return "tree"
	case OBJ_TAG:
		return "tag"
	case OBJ_OFS_DELTA:
		return "ofs-delta"
	case OBJ_REF_DELTA:
		return "ref-delta"
	default:
		return "INVALID"
	}
}

func typeByte(b byte) ObjectType {
	return ObjectType((b >> 4) & 0b0111)
}

func hasMore(b byte) bool {
	return moreBit&b > 0
}

func ReadPack(packfile string, offset int64) (ObjectType, []byte, error) {
	file, err := os.Open(packfile)
	if err != nil {
		return OBJ_INVALID, nil, err
	}
	defer file.Close()
	file.Seek(offset, io.SeekStart)
	b := make([]byte, 1)
	_, err = file.Read(b)
	if err != nil {
		return OBJ_INVALID, nil, err
	}
	// Neither size nor type is needed for now
	otype := typeByte(b[0])
	osize := uint64(b[0] & 0b1111)
	off := 4
	for hasMore(b[0]) {
		_, err = file.Read(b)
		if err != nil {
			return OBJ_INVALID, nil, err
		}
		osize |= uint64(b[0]&0b0111_1111) << off
		off += 7
	}
	// TESTING TESTING TESTING TESTING TESTING TESTING TESTING
	if otype == OBJ_OFS_DELTA {
		// TODO [delta_decoding]: Support delta reprentations in packfiles.
		// See https://stefan.saasen.me/articles/git-clone-in-haskell-from-the-bottom-up/#delta-encoding.
		// AFAIU OFS_DELTA objects encode the negative offset to the base object (in the same pack file) as a varint,
		// followed by the copy and insert instructions needed to build the target object from the base.
		for {
			n, err := file.Read(b)
			if err != nil || n == 0 || b[0] == 0 {
				return OBJ_INVALID, nil, nil
			}
			fmt.Print(string(b))
		}
	}
	o, err := Deflate(file)
	if err != nil {
		return OBJ_INVALID, nil, err
	}
	return otype, o, nil
}
