// Package decoder provides a way to decode MDX/MDD file in a native way, rather than an external python scripts.
// It can provides users with easer usage of utilizing existed MDX dictionaries.
// For more details, please to refer to:
//
//	https://github.com/zhansliu/writemdict/blob/master/fileformat.md
//	https://bitbucket.org/xwang/mdict-analysis/src
package decoder

import (
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"unicode/utf16"
)

type Header struct {
	GeneratedByEngineVersion string `xml:"GeneratedByEngineVersion,attr"`
	RequiredEngineVersion    string `xml:"RequiredEngineVersion,attr"`
	Encrypted                string `xml:"Encrypted,attr"`
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
func Decode(fileName string) error {
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
	return nil
}
