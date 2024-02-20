package gitwood

import (
	"bufio"
	"bytes"
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
// Returns -1 if no object with the givein shasum could be found.
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
	// Simple O(n)-search, can optimize with binary search later
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
	// alt: b & 0b1000_0000 > 0
	return b >= 0x80
}

var errOverflow = errors.New("binary: varint overflows a 64-bit integer")

// getOffsetVarint reads the special varint format of git offsets.
// Took me waaay too much time to realize that the Git format is not standard varint format.
// Here's the spec:
//
//	n bytes with MSB set in all but the last one.
//	The offset is then the number constructed by
//	concatenating the lower 7 bit of each byte, and
//	for n >= 2 adding 2^7 + 2^14 + ... + 2^(7*(n-1))
//	to the result.
//
// What this means is that MSBs of the resulting ints are defined first,
// in contrast to the standard varint format, where the LSBs are defined first.
func gitOffsetVarint(r io.ByteReader) (uint64, uint64, error) {
	var (
		x   uint64
		n   uint64
		a   uint64
		b   = byte(0xff)
		err error
	)
	// Concatenate the lower 7 bits of each byte.
	for hasMore(b) && n < binary.MaxVarintLen64 {
		b, err = r.ReadByte()
		n++
		if err != nil {
			return x, n, err
		}
		// Shifting by a decreasing shift makes the bits "concatenate" in the right order.
		x |= uint64(b&0x7f) << (7 * (binary.MaxVarintLen64 - n - 1))
		// The addend part (2^7 + 2^14 + ... + 2^(7*(n-1)))
		if n >= 2 {
			a += 1 << (7 * (n - 1))
		}
	}
	// Now the bits must be shifted back, and the addend must be added.
	return (x >> ((binary.MaxVarintLen64 - n - 1) * 7)) + a, n, nil
}

// uvarint is a copy of binary.Uvarint, but it returns the number of bytes read.
func uvarint(r io.ByteReader) (uint64, uint64, error) {
	var (
		x uint64
		s uint
		n uint64
	)
	for n < binary.MaxVarintLen64 {
		b, err := r.ReadByte()
		if err != nil {
			if n > 0 && err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return x, 0, err
		}
		n++
		if b < 0x80 {
			if n == binary.MaxVarintLen64-1 && b > 1 {
				return x, n, errOverflow
			}
			return x | uint64(b)<<s, n, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return x, n, errOverflow
}

// There may be other ways, but it seems safer to create a new buffer every time the offset changes.
func newBufReader(file *os.File, off uint64) (*bufio.Reader, error) {
	_, err := file.Seek(int64(off), io.SeekStart)
	if err != nil {
		return nil, err
	}
	return bufio.NewReader(file), nil
}

func (r *Repo) OpenAndReadFromPack(packfile string, off uint64) (ObjectType, []byte, error) {
	file, err := os.Open(packfile)
	if err != nil {
		return OBJ_INVALID, nil, fmt.Errorf("failed to open packfile: %w", err)
	}
	defer file.Close()
	otype, o, err := r.readFromPack(file, off)
	if err != nil {
		return OBJ_INVALID, nil, fmt.Errorf("readFromPack failed: %w", err)
	}
	return otype, o, err
}

// o is the current read offset in the file.
// This must be maintained manually, because buffers are used to read from the file.
func (r *Repo) readFromPack(file *os.File, off uint64) (ObjectType, []byte, error) {
	// Original offset
	oOff := off
	buf, err := newBufReader(file, off)
	if err != nil {
		return OBJ_INVALID, nil, err
	}
	b, err := buf.ReadByte()
	off++
	if err != nil {
		return OBJ_INVALID, nil, err
	}
	otype := typeByte(b)
	osize := uint64(b & 0b1111)
	// Can't use standard uvarint (directly) here,
	// because the LSBs are given by the four LSBs of the first byte (the byte is "shared" with the type bits).
	sizeOff := uint64(4)
	for hasMore(b) {
		b, err = buf.ReadByte()
		off++
		if err != nil {
			return OBJ_INVALID, nil, err
		}
		osize |= uint64(b&0b0111_1111) << sizeOff
		sizeOff += 7
	}
	if otype == OBJ_OFS_DELTA || otype == OBJ_REF_DELTA {
		var botype ObjectType
		var bo []byte
		if otype == OBJ_OFS_DELTA {
			buf, botype, bo, err = r.getOfsDeltaBase(file, buf, oOff, off)
		} else if otype == OBJ_REF_DELTA {
			botype, bo, err = r.getRefDeltaBase(file, buf, off)
		}
		if err != nil {
			return OBJ_INVALID, nil, fmt.Errorf("delta extraction failed: %w", err)
		}
		deltaData, err := Decompress(buf)
		if err != nil {
			return otype, nil, fmt.Errorf("failed to decompress delta data: %w", err)
		}
		// Now apply the delta
		o, err := r.applyDelta(bo, deltaData)
		if err != nil {
			return OBJ_INVALID, nil, fmt.Errorf("failed to apply delta: %w", err)
		}
		return botype, o, err
	}
	o, err := Decompress(buf)
	if err != nil {
		return otype, nil, err
	}
	return otype, o, nil
}

func (r *Repo) getOfsDeltaBase(file *os.File, buf *bufio.Reader, oOff, off uint64) (*bufio.Reader, ObjectType, []byte, error) {
	bOff, num, err := gitOffsetVarint(buf)
	off += num
	if err != nil {
		return nil, OBJ_INVALID, nil, fmt.Errorf("failed to read ofs-delta offset: %w", err)
	}
	if err != nil {
		return nil, OBJ_INVALID, nil, fmt.Errorf("failed to read ofs-delta moff: %w", err)
	}
	otype, o, err := r.readFromPack(file, oOff-bOff)
	if err != nil {
		return nil, OBJ_INVALID, nil, fmt.Errorf("failed to read delta base object: %w", err)
	}
	if otype == OBJ_OFS_DELTA || otype == OBJ_REF_DELTA {
		return nil, OBJ_INVALID, nil, fmt.Errorf("delta extraction returned a delta object")
	}
	// Create new buffer to "reset the offset".
	// This is necessary because the delta base object is read from the same file,
	// i.e. readFromPack will affect the offset of the current file.
	buf, err = newBufReader(file, off)
	if err != nil {
		return nil, OBJ_INVALID, nil, fmt.Errorf("failed to create new buffer for delta: %w", err)
	}
	return buf, otype, o, nil
}

func (r *Repo) getRefDeltaBase(file *os.File, buf *bufio.Reader, off uint64) (ObjectType, []byte, error) {
	var shasum [20]byte
	num, err := buf.Read(shasum[:])
	if err != nil {
		return OBJ_INVALID, nil, fmt.Errorf("failed to read ref-delta shasum: %w", err)
	}
	off += uint64(num)
	// Optimization: All deltas refer to objects in the same pack,
	// so there's no need to search all packs.
	// From git docs:
	// > When stored on disk (however), the pack should be self contained to avoid cyclic dependency.
	otype, o, err := r.searchAllPacks(hex.EncodeToString(shasum[:]))
	if err != nil {
		return OBJ_INVALID, nil, fmt.Errorf("failed to read delta shasum: %w", err)
	}
	if otype == OBJ_OFS_DELTA || otype == OBJ_REF_DELTA {
		return OBJ_INVALID, nil, fmt.Errorf("delta extraction returned a delta object")
	}
	return otype, o, nil
}

func deltaCopy(target *[]byte, base []byte, deltaData []byte, inst byte) (uint64, error) {
	const offsetBits = 4
	const sizeBits = 3
	var num uint64
	var offset uint32
	testbit := byte(0b_000_0001)
	for i := 0; i < offsetBits; i++ {
		if inst&(testbit<<i) == 0 {
			continue
		}
		offset |= uint32(deltaData[num]) << (i * 8)
		num++
	}
	var size uint32
	testbit = byte(0b_001_0000)
	for i := 0; i < sizeBits; i++ {
		if inst&(testbit<<i) == 0 {
			continue
		}
		size |= uint32(deltaData[num]) << (i * 8)
		num++
	}
	*target = append(*target, base[offset:offset+size]...)
	return num, nil
}

func deltaInsert(target *[]byte, deltaData []byte, inst byte) (uint64, error) {
	num := uint8(inst)
	if uint8(len(deltaData)) < num {
		return 0, fmt.Errorf("delta insert: not enough insert data")
	}
	*target = append(*target, deltaData[:num]...)
	return uint64(num), nil
}

func (r *Repo) applyDelta(base, deltaData []byte) ([]byte, error) {
	var target []byte
	var n, off uint64
	var err error
	buf := bytes.NewBuffer(deltaData)
	// base size
	_, n, err = uvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read base size: %w", err)
	}
	off += n
	// target size
	_, n, err = uvarint(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read target size: %w", err)
	}
	off += n
	for off < uint64(len(deltaData)) {
		// Get the instruction byte
		b := deltaData[off]
		off++
		if b&0b1000_0000 > 0 {
			n, err = deltaCopy(&target, base, deltaData[off:], b)
		} else {
			n, err = deltaInsert(&target, deltaData[off:], b)
		}
		if err != nil {
			return nil, fmt.Errorf("delta instruction failed: %w", err)
		}
		off += n
	}
	return target, nil
}
