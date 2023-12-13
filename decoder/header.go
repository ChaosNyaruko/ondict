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
	"fmt"
	"hash/adler32"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"unicode/utf16"
)

type MDict struct {
	Encrypted int8
	Encoding  string
	RegCode   string
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

// TODO: rename it
func (m *MDict) Decode(fileName string) error {
	name, err := filepath.Abs(fileName)
	if err != nil {
		return err
	}
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()

	var headerLen uint32
	if err := binary.Read(file, binary.BigEndian, &headerLen); err != nil {
		return err
	}
	fmt.Printf("headerLen: %v\n", headerLen)

	// It must be even, cuz head_str is UTF-16 encoded
	if headerLen%2 != 0 {
		log.Fatalf("headerLen must be even, but got %v", headerLen)
	}
	headerSize := headerLen / 2
	var headerRunes = make([]uint16, headerSize)

	if err := binary.Read(file, binary.LittleEndian, headerRunes); err != nil {
		return err
	}
	headerXML := string(utf16.Decode(headerRunes))
	fmt.Println("header as XML", headerXML)

	var header Header
	if err := xml.Unmarshal([]byte(headerXML), &header); err != nil {
		return err
	}

	fmt.Printf("header as structured: %+v\n", header)
	m.Encoding = header.Encoding

	// TODO: ADLER32 checksum of header, we just read it, remember to check it
	var checksum uint32
	if err := binary.Read(file, binary.LittleEndian, &checksum); err != nil {
		return err
	}
	fmt.Printf("header checksum: %+v\n", checksum)

	encrypt, err := strconv.Atoi(header.Encrypted)
	if err != nil {
		return err
	}
	m.Encrypted = int8(encrypt)
	if err := m.decodeKeyWordSection(file); err != nil {
		return fmt.Errorf("decode keyword section: %v", err)
	}
	if err := m.decodeRecordSection(file); err != nil {
		return fmt.Errorf("decode record section: %v", err)
	}
	return nil
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

	type keywordSectionHeader struct {
		NumBlock          uint64
		NumEntries        uint64
		KeyIndexDecompLen uint64
		KeyIndexCompLen   uint64
		KeyBlockLen       uint64
	}
	var header keywordSectionHeader
	if err := binary.Read(fd, binary.BigEndian, &header); err != nil {
		return err
	}
	if m.Encrypted&1 != 0 {
		// TODO: the first 40 bytes might be encrypted
	}
	var keywordHeaderChecksum [4]byte

	if err := binary.Read(fd, binary.BigEndian, &keywordHeaderChecksum); err != nil {
		return err
	}
	fmt.Printf("raw keyword section header: %#v, checksum: %v\n", header, keywordHeaderChecksum)

	keyIndexEncrypted := (m.Encrypted & 2) != 0
	var keyIndexDecompressed []byte
	// Decrypt keyword Index if encrypted
	// After this, we will get compressed keyword Index
	if keyIndexEncrypted {
		log.Printf("keyword index encrypted, decrypt it")
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
		compType := keyIndexEncrypted[:4]
		compressedChecksum := keyIndexEncrypted[4:8]
		log.Printf("len(keyIndexEncrypted): %v, %v:%v:%v", len(keyIndexEncrypted), keyIndexEncrypted[:4], keyIndexEncrypted[4:8], keyIndexEncrypted[8:])
		keyIndexDecrypted := keywordIndexDecrypt(keyIndexEncrypted)
		log.Printf("len(keyIndexDecrypted): %v, %v:%v:%v", len(keyIndexDecrypted), keyIndexDecrypted[:4], keyIndexDecrypted[4:8], keyIndexDecrypted[8:])
		keyIndexDecompressed = decompress(compType, compressedChecksum, keyIndexDecrypted[8:])
	} else {
		// TODO: the same decompression as in the encrypted version, just not including the decrypt process.
	}

	log.Printf("keyIndexDecompressed len: %d", len(keyIndexDecompressed))
	if len(keyIndexDecompressed) != int(header.KeyIndexDecompLen) {
		// TODO: according to the exsited Python implementation, this fields only existed in version >= 2
		log.Fatalf("the length of decompressed part is wrong: expected: %v, got :%v", header.KeyIndexDecompLen, len(keyIndexDecompressed))
	}
	// Decode the decompressed keyword index part
	r := bytes.NewReader(keyIndexDecompressed)
	type size struct {
		comp   uint64
		decomp uint64
	}

	var sizes []size
	for i := 0; i < int(header.NumBlock); i++ {
		var numEntries uint64
		if err := binary.Read(r, binary.BigEndian, &numEntries); err != nil {
			return err
		}
		if m.Encoding == "UTF-16" {
			// TODO: the "size"s are halved
		}
		var firstSize uint16 // the number of "basic units" for the encoding of the first word
		if err := binary.Read(r, binary.BigEndian, &firstSize); err != nil {
			return err
		}
		firstWord := make([]byte, firstSize+1) // TODO: why the "+1"?
		if err := binary.Read(r, binary.BigEndian, firstWord); err != nil {
			return err
		}
		// TODO: []byte to utf-16 encoded string
		fmt.Printf("the first word of index[%d], [len:%d]%v\n", i, firstSize, string(firstWord))

		var lastSize uint16 // the number of "basic units" for the encoding of the first word
		if err := binary.Read(r, binary.BigEndian, &lastSize); err != nil {
			return err
		}
		log.Printf("the last word size of index[%d], %v\n", i, lastSize)
		if m.Encoding == "UTF-16" {
			lastSize *= 2
		}
		lastWord := make([]byte, lastSize+1) // TODO: why the "+1"? text_term --> termination? For version >=2
		if err := binary.Read(r, binary.BigEndian, lastWord); err != nil {
			return err
		}
		// TODO: []byte to utf-16 encoded string
		fmt.Printf("the last word of index[%d], %v\n", i, string(lastWord))

		var compSize uint64
		if err := binary.Read(r, binary.BigEndian, &compSize); err != nil {
			return err
		}
		log.Printf("comp len of key_blocks[%d], %v\n", i, compSize)

		var decompSize uint64
		if err := binary.Read(r, binary.BigEndian, &decompSize); err != nil {
			return err
		}
		log.Printf("decomp len of key_blocks[%d], %v\n", i, decompSize)
		sizes = append(sizes, size{compSize, decompSize})
	}

	// decode key blocks
	for i, s := range sizes {
		log.Printf("decoding [%d]th key block", i)
		compressed := make([]byte, s.comp)
		if err := binary.Read(fd, binary.BigEndian, compressed); err != nil {
			return err
		}
		decompressed := decompress(compressed[:4], compressed[4:8], compressed[8:])
		if len(decompressed) != int(s.decomp) {
			log.Fatalf("decomp len not as expected!")
		}
	}
	return nil
}

func (m *MDict) decodeRecordSection(fd io.Reader) error {
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
	log.Printf("type: %v, checksum: %v", compType, checksum)
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
