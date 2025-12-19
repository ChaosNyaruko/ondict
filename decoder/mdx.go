// Package decoder provides a way to decode MDX/MDD file in a native way, rather than an external python scripts.
// It can provides users with easer usage of utilizing existed MDX dictionaries.
// For more details, please to refer to:
//
//	https://github.com/zhansliu/writemdict/blob/master/fileformat.md
//	https://bitbucket.org/xwang/mdict-analysis/src
package decoder

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/adler32"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/ChaosNyaruko/ondict/util"
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
)

type keyOffset struct {
	offset uint64
	key    []byte
}

type MDict struct {
	t          string
	header     Header
	encrypted  int8
	encoding   string
	regCode    string
	numEntries int
	keys       []keyOffset
	records    []byte
	lazyOffset int

	once   sync.Once
	keymap map[string][]uint64 // key: key value: the index of the key in MDict.keys

	file             *os.File // the raw fd, to avoid store all "records" bytes, they are memory-head
	recordHeader     recordSection
	recordBlockSizes []recordBlock
}

type Header struct {
	GeneratedByEngineVersion string `xml:"GeneratedByEngineVersion,attr"`
	RequiredEngineVersion    string `xml:"RequiredEngineVersion,attr"`
	Encrypted                string `xml:"Encrypted,attr"`
	Encoding                 string `xml:"Encoding,attr"`
	Format                   string `xml:"Format,attr"`
	CreationDate             string `xml:"CreationDate,attr"`
	Compact                  string `xml:"Compact,attr"`
	Compat                   string `xml:"Compat,attr"`
	KeyCaseSensitive         string `xml:"KeyCaseSensitive,attr"`
	Description              string `xml:"Description,attr"`
	Title                    string `xml:"Title,attr"`
	DataSourceFormat         string `xml:"DataSourceFormat,attr"`
	StyleSheet               string `xml:"StyleSheet,attr"`
	RegisterBy               string `xml:"RegisterBy,attr"`
	RegCode                  string `xml:"RegCode,attr"`
}

func (m *MDict) Get(word string) string {
	m.DumpKeys()
	log.Tracef("Get %v from MDict", word)
	var res []string
	for _, offset := range m.keymap[word] {
		exp := m.decodeString(m.ReadAtOffset(int(offset)))
		if link, ok := strings.CutPrefix(exp, "@@@LINK="); ok && string(link)[len(link)-1] == 0 {
			link = link[:len(link)-3] // ending with \r\n\x00
			exp = fmt.Sprintf(`
See <a class=Crossrefto href="/dict?query=%s&engine=mdx&format=html">%s</a> for more
</div>`,

				url.QueryEscape(link), link)
		}

		res = append(res, exp)
	}
	return strings.Join(res, "<br/>")
}

func (m *MDict) DumpKeys() {
	m.once.Do(func() {
		m.keymap = make(map[string][]uint64, m.numEntries)
		if m.numEntries != len(m.keys) {
			log.Warnf("dumpKeys: num entries number does not match: keys have[%d], numEntries:%v",
				len(m.keys), m.numEntries)
		}
		bar := progressbar.Default(int64(m.numEntries), fmt.Sprintf("dumping keys for %v(%v)", m.header.Title, m.t))
		for i, k := range m.keys {
			key := m.decodeString(k.key)
			_ = bar.Add(1)
			m.keymap[key] = append(m.keymap[key], uint64(i))
		}
	})
}

func (m *MDict) Keys() []string {
	m.DumpKeys()
	res := make([]string, 0, len(m.keymap))
	for k := range m.keymap {
		res = append(res, k)
	}
	return res
}

func (m *MDict) Close() error {
	return m.file.Close()
}

func (m *MDict) Decode(fileName string, fzf bool) error {
	start := time.Now()
	defer func() {
		log.Debugf("decode cost: %v", time.Since(start))
	}()
	name, err := filepath.Abs(fileName)
	if err != nil {
		return err
	}
	if m.t = filepath.Ext(name); m.t != ".mdx" && m.t != ".mdd" {
		return fmt.Errorf("unexpected file ext %q", m.t)
	}
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	m.file = file

	var headerLen uint32
	if err := binary.Read(file, binary.BigEndian, &headerLen); err != nil {
		return err
	}
	m.lazyOffset += 4
	// fmt.Printf("headerLen: %v\n", headerLen)

	// It must be even, cuz head_str is UTF-16 encoded
	if headerLen%2 != 0 {
		log.Fatalf("headerLen must be even, but got %v", headerLen)
	}
	var headerBytes = make([]uint8, headerLen)

	if err := binary.Read(file, binary.LittleEndian, headerBytes); err != nil {
		return err
	}
	m.lazyOffset += int(headerLen)

	var checksum uint32
	if err := binary.Read(file, binary.LittleEndian, &checksum); err != nil {
		return err
	}
	m.lazyOffset += 4
	if adler32.Checksum(headerBytes) != checksum {
		return fmt.Errorf("the checksum of header str does not match")
	}

	headerSize := headerLen / 2
	var headerRunes = make([]uint16, headerSize)
	h := bytes.NewReader(headerBytes)
	if err := binary.Read(h, binary.LittleEndian, headerRunes); err != nil {
		return err
	}

	headerXML := string(utf16.Decode(headerRunes))

	var header Header
	if err := xml.Unmarshal([]byte(headerXML), &header); err != nil {
		return err
	}

	log.Debugf("header as structured: %+v\n", header)
	if header.GeneratedByEngineVersion != "2.0" {
		log.Fatalf("TODO: Only 2.0 Version is supported!, but the input MDX file was generated by engine: %v",
			header.GeneratedByEngineVersion)
	}
	m.header = header
	m.encoding = header.Encoding
	if m.t == ".mdd" {
		m.encoding = "UTF-16"
	}

	encrypt, err := strconv.Atoi(header.Encrypted)
	if err != nil {
		return err
	}
	m.encrypted = int8(encrypt)

	if err := m.decodeKeyWordSection(file); err != nil {
		return fmt.Errorf("decode keyword section: %v", err)
	}
	// offset, err := file.Seek(0, io.SeekCurrent)
	// log.Debugf("offset of Record start: %v, err: %v", offset, err)
	x := time.Now()
	if err := m.decodeRecordSection(file, fzf); err != nil {
		return fmt.Errorf("decode record section: %v", err)
	}
	log.Debugf("decode record cost: %v", time.Since(x))
	// The reader should be at EOF now
	var eof = make([]byte, 1)
	if n, err := file.Read(eof); err != nil && n == 0 {
		// log.Debugf("n: %v, err: %v", n, err)
		if errors.Is(err, io.EOF) {
			return nil
		}
	} else {
		// return fmt.Errorf("the reader should be empty now!")
		log.Debugf("m.lazyOffset: %v", m.lazyOffset)
	}
	return nil
}

func (m *MDict) decodeString(b []byte) string {
	if m.encoding == "UTF-16" {
		runes := make([]uint16, len(b)/2)
		if err := binary.Read(bytes.NewBuffer(b), binary.LittleEndian, runes); err != nil {
			panic(err)
		}
		return string(utf16.Decode(runes))
	}
	return string(b)
}

func (m *MDict) fetchNthRecordBlock(i int, bytesBefore int) []byte {
	log.Tracef("fetchNthRecordBlock: %d, bytesBefore: %d, CompSize: %v", i, bytesBefore, m.recordBlockSizes[i].CompSize)
	compressed1 := make([]byte, m.recordBlockSizes[i].CompSize)
	n, err := m.file.ReadAt(compressed1, int64(m.lazyOffset+bytesBefore))
	if err != nil {
		log.Fatalf("read at err %v", err)
	}
	if n != int(m.recordBlockSizes[i].CompSize) {
		log.Fatalf("read %v bytes, but expected %v bytes", n, m.recordBlockSizes[i].CompSize)
	}
	if n == 0 {
		return nil
	}
	decompressed := decompress(compressed1[:4], compressed1[4:8], compressed1[8:])
	if len(decompressed) != int(m.recordBlockSizes[i].DecompSize) {
		log.Fatalf("decompressed length does not equal to expected")
	}
	return decompressed
}

func (m *MDict) ReadAtOffset(index int) []byte {
	log.Tracef("m.keys len: %v, try to access: %v, nblock: %d[%d]", len(m.keys), index, len(m.recordBlockSizes), m.recordHeader.NumBlocks)
	if index >= len(m.keys) {
		log.Fatalf("invalid index for %s.%s", m.header.Title, m.t)
	}

	var offset1 uint64 = math.MaxUint64
	if index < len(m.keys)-1 {
		offset1 = m.keys[index+1].offset
	}
	offset := m.keys[index].offset
	total := uint64(0)
	totalDecomp := uint64(0)
	pre := -1
	preDecomp := -1
	iBlock := -1
	i1Block := -1
	for i := uint64(0); i < m.recordHeader.NumBlocks; i++ {
		log.Tracef("%d block: %v->%v", i, m.recordBlockSizes[i].CompSize, m.recordBlockSizes[i].DecompSize)
		pre = int(total)
		preDecomp = int(totalDecomp)
		total += m.recordBlockSizes[i].CompSize
		totalDecomp += m.recordBlockSizes[i].DecompSize
		if offset < totalDecomp {
			iBlock = int(i)
			if offset1 < totalDecomp {
				i1Block = iBlock
			} else {
				i1Block = iBlock + 1
			}
			break
		}
	}
	log.Tracef("index: %v, iBlock: %d, i1Block: %d, pre: %d, preDecomp: %d, total: %d, totalDecomp: %d, offset:%d",
		index, iBlock, i1Block, pre, preDecomp, total, totalDecomp, m.lazyOffset)
	if iBlock < 0 {
		log.Fatalf("doesn't find a valid block for offset %d, index: %d", iBlock, index)
	}
	decompressed := m.fetchNthRecordBlock(iBlock, pre)
	offset = offset - uint64(preDecomp)
	offset1 = offset1 - uint64(preDecomp)
	if iBlock == i1Block { // | ---*--*- | -------- |
		return decompressed[offset:offset1]
	} else if i1Block < len(m.recordBlockSizes) { // | ---*--- | -*-- |
		decompressed1 := m.fetchNthRecordBlock(i1Block, int(total))
		decompressed = append(decompressed, decompressed1...)
		return decompressed[offset:offset1]
	}
	// i1Block == len(m.recordBlockSizes) , index is inside the last block | ---*--- |
	return decompressed[offset:]
}

// DumpDict may cost quite a long time, use it when you actually need the whole data
func (m *MDict) DumpDict() (map[string]string, error) {
	if m.t != ".mdx" {
		return nil, fmt.Errorf("The dict should be the MDX file, not %v", m.t)
	}
	start := time.Now()
	defer func() {
		log.Debugf("dump dict cost: %v", time.Since(start))
	}()
	res := make(map[string]string, m.numEntries)
	total := 0
	bar := progressbar.Default(int64(len(m.keys)), fmt.Sprintf("Dumping dict: %v", m.header.Title))
	for i, k := range m.keys {
		res[m.decodeString(k.key)] = m.decodeString((m.ReadAtOffset(i)))
		total += 1
		bar.Add(1)
	}
	if total != m.numEntries {
		return nil, fmt.Errorf("the keys not suffice, got: %v, expected: %v", total, m.numEntries)
	}
	return res, nil
}

func decryptRegCode(regCode []byte, id []byte) []byte {
	idDigest := ripemd128(id)
	s20 := salsa20(regCode, idDigest, [8]byte{}, 8)
	return s20
}

func (m *MDict) salsaDecrypt(data []byte, id []byte, regCode []byte) ([]byte, error) {
	key := decryptRegCode(regCode, id) // [32]byte
	return salsa20(data, key, [8]byte{}, 8), nil
}

func (m *MDict) decodeKeyWordSection(fd io.Reader) error {
	// num_blocks	8 bytes	Number of items in key_blocks. Big-endian. Possibly encrypted, see below.
	// num_entries	8 bytes	Total number of keywords. Big-endian. Possibly encrypted, see below.
	// key_index_decomp_len	8 bytes	Number of bytes in decompressed version of key_index. Big-endian. Possibly encrypted, see below.
	// key_index_comp_len	8 bytes	Number of bytes in compressed version of key_index (including the comp_type and checksum parts). Big-endian. Possibly encrypted, see below.
	// key_blocks_len	8 bytes	Total number of bytes taken up by key_blocks. Big-endian. Possibly encrypted, see below.
	// checksum	4 bytes	ADLER32 checksum of the preceding 40 bytes. Big-endian.
	// key_index	varying	The keyword index, compressed and possibly encrypted. See below.
	// key_blocks[0]	varying	A compressed block containing keywords, compressed. See below.
	// ...	...	...
	// key_blocks[num_blocks-1]	varying	...

	log.Debugf("decoding keyword section")
	var rawHeader = make([]byte, 40)
	if err := binary.Read(fd, binary.BigEndian, rawHeader); err != nil {
		return err
	}
	m.lazyOffset += 40
	var err error
	h := bytes.NewReader(rawHeader)
	type keywordSectionHeader struct {
		NumBlock          uint64
		NumEntries        uint64
		KeyIndexDecompLen uint64
		KeyIndexCompLen   uint64
		KeyBlockLen       uint64
	}
	if m.encrypted&1 != 0 {
		log.Fatal("TODO: keyword header encrypted, salsa20 not supported yet")
		rawHeader, err = m.salsaDecrypt(rawHeader, []byte("TODO: userid"), []byte(m.regCode))
		if err != nil {
			return err
		}
	}

	var header keywordSectionHeader
	if err := binary.Read(h, binary.BigEndian, &header); err != nil {
		return err
	}
	var keywordHeaderChecksum [4]byte

	if err := binary.Read(fd, binary.BigEndian, &keywordHeaderChecksum); err != nil {
		return err
	}
	m.lazyOffset += 4

	if adler32.Checksum(rawHeader) != binary.BigEndian.Uint32(keywordHeaderChecksum[:]) {
		return fmt.Errorf("the checksum of keyword header does not match")
	}

	m.numEntries = int(header.NumEntries)
	log.Debugf("key header %#v", header)

	// encrypted by the following C function
	// #define SWAPNIBBLE(byte) (((byte)>>4) | ((byte)<<4))
	// void encrypt(unsigned char* buf, size_t buflen, unsigned char* key, size_t keylen) {
	// 	unsigned char prev=0x36;
	// 	for(size_t i=0; i < buflen; i++) {
	// 		buf[i] = SWAPNIBBLE(buf[i] ^ ((unsigned char)i) ^ key[i%keylen] ^ previous);
	// 		previous = buf[i];
	// 	}
	// }
	keyIndexEncrypted := make([]byte, header.KeyIndexCompLen)
	if err := binary.Read(fd, binary.BigEndian, keyIndexEncrypted); err != nil {
		return err
	}
	m.lazyOffset += int(header.KeyIndexCompLen)
	compType := keyIndexEncrypted[:4]
	compressedChecksum := keyIndexEncrypted[4:8]
	// log.Debugf("len(keyIndexEncrypted): %v, %v:%v:%v", len(keyIndexEncrypted), keyIndexEncrypted[:4], keyIndexEncrypted[4:8], keyIndexEncrypted[8:])
	keyIndexDecrypted := keyIndexEncrypted
	encrypted := (m.encrypted & 2) != 0
	if encrypted {
		// Decrypt keyword Index if encrypted
		// After this, we will get compressed keyword Index
		log.Debugf("keyword index encrypted, decrypt it")
		keyIndexDecrypted = keywordIndexDecrypt(keyIndexEncrypted)
	}
	// log.Debugf("len(keyIndexDecrypted): %v, %v:%v:%v", len(keyIndexDecrypted), keyIndexDecrypted[:4], keyIndexDecrypted[4:8], keyIndexDecrypted[8:])
	keyIndexDecompressed := decompress(compType, compressedChecksum, keyIndexDecrypted[8:])

	// log.Debugf("keyIndexDecompressed len: %d", len(keyIndexDecompressed))
	if len(keyIndexDecompressed) != int(header.KeyIndexDecompLen) {
		// TODO: according to the exsited Python implementation, this fields only existed in version >= 2
		log.Fatalf("the length of decompressed part is wrong: expected: %v, got :%v", header.KeyIndexDecompLen, len(keyIndexDecompressed))
	}
	// Decode the decompressed keyword index part
	r := bytes.NewReader(keyIndexDecompressed)
	type keyBlock struct {
		comp       uint64
		decomp     uint64
		numEntries uint64
		firstWord  []byte
		lastWord   []byte
	}

	var keyBlocks []keyBlock
	totalEntries := 0
	for i := 0; i < int(header.NumBlock); i++ {
		var numEntries uint64
		if err := binary.Read(r, binary.BigEndian, &numEntries); err != nil {
			return err
		}
		totalEntries += int(numEntries)
		var firstSize uint16 // the number of "basic units" for the encoding of the first word
		if err := binary.Read(r, binary.BigEndian, &firstSize); err != nil {
			return err
		}
		firstSize += 1 // why the "+1"? text_term --> termination? For version >=2
		if m.encoding == "UTF-16" {
			firstSize = firstSize * 2
		}
		firstWord := make([]byte, firstSize) // including the terminator
		if err := binary.Read(r, binary.BigEndian, firstWord); err != nil {
			return err
		}
		var lastSize uint16 // the number of "basic units" for the encoding of the first word
		if err := binary.Read(r, binary.BigEndian, &lastSize); err != nil {
			return err
		}
		lastSize += 1
		if m.encoding == "UTF-16" {
			lastSize = lastSize * 2
		}
		lastWord := make([]byte, lastSize)
		if err := binary.Read(r, binary.BigEndian, lastWord); err != nil {
			return err
		}

		var compSize uint64
		if err := binary.Read(r, binary.BigEndian, &compSize); err != nil {
			return err
		}
		// log.Debugf("comp len of key_blocks[%d], %v\n", i, compSize)

		var decompSize uint64
		if err := binary.Read(r, binary.BigEndian, &decompSize); err != nil {
			return err
		}
		// log.Debugf("decomp len of key_blocks[%d], %v\n", i, decompSize)
		keyBlocks = append(keyBlocks, keyBlock{compSize, decompSize, numEntries, firstWord, lastWord})
	}

	log.Debugf("total entries: %v", totalEntries)
	// decode key blocks
	for _, b := range keyBlocks {
		// log.Debugf("decoding [%d]th key block", i)
		compressed := make([]byte, b.comp)
		if err := binary.Read(fd, binary.BigEndian, compressed); err != nil {
			return err
		}
		m.lazyOffset += int(b.comp)
		decompressed := decompress(compressed[:4], compressed[4:8], compressed[8:])
		if len(decompressed) != int(b.decomp) {
			log.Fatalf("decomp len not as expected!")
		}
		m.splitKeyBlock(decompressed, int(b.numEntries))
	}

	return nil
}

func (m *MDict) splitKeyBlock(b []byte, keyNum int) {
	// log.Debugf("block %d, num: %d, %v", index, keyNum, b)
	delimiterWidth := 1
	delimiter := []byte{0x00}
	if m.encoding == "UTF-16" {
		delimiterWidth = 2
		delimiter = []byte{0x00, 0x00}
	}
	p := 0
	for i := 0; i < keyNum; i++ {
		p += 8
		offset := binary.BigEndian.Uint64(b[p-8 : p])
		keyBytes := make([]byte, 0)
		for p < len(b) && (!reflect.DeepEqual(b[p:p+delimiterWidth], delimiter)) { // TODO: performance
			keyBytes = append(keyBytes, b[p:p+delimiterWidth]...)
			p += delimiterWidth
		}
		p += delimiterWidth
		// log.Debugf("splitKeyBlock key[%v][%v] at offset [%d]\n", len(m.keys), m.decodeString(keyBytes), offset)
		m.keys = append(m.keys, keyOffset{offset, keyBytes})
	}
}

type recordSection struct {
	NumBlocks  uint64
	NumEntries uint64
	IndexLen   uint64
	BlocksLen  uint64
}

type recordBlock struct {
	CompSize   uint64
	DecompSize uint64
}

func (m *MDict) decodeRecordSection(fd io.Reader, lazy bool) error {
	var recordHeader recordSection
	if err := binary.Read(fd, binary.BigEndian, &recordHeader); err != nil {
		return err
	}
	m.lazyOffset += 8 * 4
	// log.Debugf("record header: %#v", recordHeader)
	if int(recordHeader.NumEntries) != m.numEntries {
		// The number of blocks does NOT need to be equal the number of keyword blocks. Big-endian.
		// But the number of entries should be EQUAL to keyword_sect.num_entries. Big-endian.
		log.Fatalf("the num of entries does not match")
	}
	if recordHeader.IndexLen != recordHeader.NumBlocks*16 {
		log.Fatalf("the index len violates its definition, check the MDX file please")
	}
	m.recordHeader = recordHeader

	records := make([]recordBlock, recordHeader.NumBlocks)
	total := 0
	totalDecomp := 0
	for i := uint64(0); i < recordHeader.NumBlocks; i++ {
		// log.Debugf("decoding [%d]th records sizes", i)
		if err := binary.Read(fd, binary.BigEndian, &records[i]); err != nil {
			return err
		}
		m.lazyOffset += 8 * 2
		total += int(records[i].CompSize)
		totalDecomp += int(records[i].DecompSize)
	}
	if total != int(recordHeader.BlocksLen) {
		log.Fatalf("the block len does not match")
	}
	m.recordBlockSizes = records
	log.Debugf("decodeRecordSection: %d<-%d", len(m.recordBlockSizes), len(records))
	m.records = make([]byte, 0, totalDecomp)
	if lazy { // FIXME: xx
		return nil
	}
	// decompress record blocks
	for i := uint64(0); i < recordHeader.NumBlocks; i++ {
		compressed := make([]byte, records[i].CompSize)
		if err := binary.Read(fd, binary.BigEndian, compressed); err != nil {
			return err
		}
		decompressed := decompress(compressed[:4], compressed[4:8], compressed[8:])
		if len(decompressed) != int(records[i].DecompSize) {
			log.Fatalf("decompressed length does not equal to expected")
		}
		m.records = append(m.records, decompressed...)
	}
	return nil
}

func keywordIndexDecrypt(data []byte) []byte {
	key := make([]byte, 4)
	copy(key, data[4:8])
	key = append(key, 0x95, 0x36, 0x00, 0x00)
	key = ripemd128(key)
	x := make([]byte, len(data))
	// The first 8 bytes are **compress type** and **check_sum**
	copy(x, data)
	previous := byte(0x36)

	b := x[8:]
	for i := 0; i < len(b); i++ {
		t := (b[i]>>4 | b[i]<<4) & 0xff
		t = t ^ previous ^ (byte(i) & 0xff) ^ key[i%len(key)]
		previous = b[i]
		b[i] = t
	}
	return x
}

func decompress(compType []byte, checksum []byte, before []byte) []byte {
	// log.Debugf("type: %v, checksum: %v", compType, checksum)
	decompressed := bytes.NewBuffer([]byte{})
	in := bytes.NewReader(before)
	switch compType[0] {
	case 0: // uncompressed, do nothing
		io.Copy(decompressed, in)
	case 1: // TODO: lzo compressed
	case 2: // zlib compressed
		if r, err := zlib.NewReader(in); err != nil {
			log.Fatalf("zlib decompress err: %v", err)
		} else {
			io.Copy(decompressed, r)
			r.Close()
		}
	}
	res := decompressed.Bytes()
	if adler32.Checksum(res) != binary.BigEndian.Uint32(checksum) {
		log.Fatalf("checksum not match for decompress! expected: %v", binary.BigEndian.Uint32(checksum))
	}
	return res
}

func (m *MDict) DumpData() error {
	if m.t != ".mdd" {
		return fmt.Errorf("The dict should be the MDX file, not %v", m.t)
	}
	bar := progressbar.Default(int64(m.numEntries), fmt.Sprintf("dumping mdd entries [%s%s]", m.header.Title, m.t))
	start := time.Now()
	defer func() {
		log.Debugf("dump data cost: %v", time.Since(start))
	}()
	m.DumpKeys()
	total := 0
	for i, k := range m.keys {
		fname := m.decodeString(k.key)
		if len(fname) > 0 {
			// strip the leading "\"
			r, size := utf8.DecodeRuneInString(fname)
			if r != '\\' {
				log.Warnf("illegal fname: %q", fname)
				continue
			}
			fname = fname[size:]
			fname = strings.ReplaceAll(fname, "\\", "/")
			log.Tracef("fname: %q, %q", fname, filepath.Dir(fname))
		}
		path := filepath.Join(util.TmpDir(), filepath.Dir(fname))
		if err := os.MkdirAll(path, 0o755); err != nil {
			log.Fatalf("make dir for %v err: %v", path, err)
		}
		fname = filepath.Join(util.TmpDir(), fname)
		if file, err := os.Create(fname); err != nil {
			log.Fatalf("open %v err: %v", fname, err)
		} else {
			n, err := file.Write(m.ReadAtOffset(i))
			log.Tracef("DumpData [%d] to file: %v, n: %v, err: %v", i, fname, n, err)
			file.Close()
		}

		total += 1
		bar.Add(1)
	}
	if total != m.numEntries {
		return fmt.Errorf("the keys not suffice")
	}
	runtime.GC()
	debug.FreeOSMemory()
	log.Debugf("[after DumpData]FreeOSMemory...")

	return nil
}
