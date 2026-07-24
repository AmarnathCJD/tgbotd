package fileid

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
)

type FileType uint32

const (
	FTThumbnail       FileType = 0
	FTProfilePhoto    FileType = 1
	FTPhoto           FileType = 2
	FTVoiceNote       FileType = 3
	FTVideo           FileType = 4
	FTDocument        FileType = 5
	FTEncrypted       FileType = 6
	FTTemp            FileType = 7
	FTSticker         FileType = 8
	FTAudio           FileType = 9
	FTAnimation       FileType = 10
	FTEncryptedThumb  FileType = 11
	FTWallpaper       FileType = 12
	FTVideoNote       FileType = 13
	FTSecureRaw       FileType = 14
	FTSecure          FileType = 15
	FTBackground      FileType = 16
	FTDocumentAsFile  FileType = 17
	FTRingtone        FileType = 18
	FTCallLog         FileType = 19
	FTPremiumGiftcode FileType = 20
	FTLivePhoto       FileType = 21
)

type Info struct {
	DC         int32
	Type       FileType
	ID         int64
	AccessHash int64
	FileRef    []byte
	VolumeID   int64
	LocalID    int32
	PhotoSize  string
	SourceType int
}

func (i *Info) Encode() string {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, i.ID)
	binary.Write(buf, binary.LittleEndian, i.AccessHash)
	binary.Write(buf, binary.LittleEndian, i.DC)
	binary.Write(buf, binary.LittleEndian, uint32(i.Type))
	if len(i.FileRef) > 0 {
		buf.WriteByte(byte(len(i.FileRef)))
		buf.Write(i.FileRef)
	}
	if i.PhotoSize != "" {
		buf.WriteByte(byte(len(i.PhotoSize)))
		buf.WriteString(i.PhotoSize)
	}
	buf.WriteByte(4)
	buf.WriteByte(30)
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(rleEncode(buf.Bytes()))
}

func Decode(s string) (*Info, error) {
	raw, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(s)
	if err != nil {
		raw, err = base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("fileid: base64 decode: %w", err)
		}
	}
	raw = rleDecode(raw)
	if len(raw) < 8+8+4+4+1+1 {
		return nil, errors.New("fileid: too short")
	}
	body := raw[:len(raw)-2]
	r := bytes.NewReader(body)
	i := &Info{}
	if err := binary.Read(r, binary.LittleEndian, &i.ID); err != nil {
		return nil, fmt.Errorf("fileid: ID: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &i.AccessHash); err != nil {
		return nil, fmt.Errorf("fileid: AccessHash: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &i.DC); err != nil {
		return nil, fmt.Errorf("fileid: DC: %w", err)
	}
	var t uint32
	if err := binary.Read(r, binary.LittleEndian, &t); err != nil {
		return nil, fmt.Errorf("fileid: Type: %w", err)
	}
	i.Type = FileType(t &^ 0xff000000)
	if r.Len() > 0 {
		refLen, err := r.ReadByte()
		if err == nil && refLen > 0 && int(refLen) <= r.Len() {
			i.FileRef = make([]byte, refLen)
			if _, err := r.Read(i.FileRef); err != nil {
				return nil, fmt.Errorf("fileid: FileRef: %w", err)
			}
		}
	}
	return i, nil
}

func (i *Info) UniqueID() string {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint32(i.Type))
	binary.Write(buf, binary.LittleEndian, i.ID)
	if i.PhotoSize != "" {
		buf.WriteByte(byte(len(i.PhotoSize)))
		buf.WriteString(i.PhotoSize)
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(buf.Bytes())
}

// rleEncode compresses zero runs. Format: every zero byte is followed by a
// count byte (may be 1 for a single-zero "run"). Non-zero bytes pass through.
func rleEncode(in []byte) []byte {
	out := make([]byte, 0, len(in))
	i := 0
	for i < len(in) {
		if in[i] == 0 {
			j := i
			for j < len(in) && in[j] == 0 && (j-i) < 255 {
				j++
			}
			out = append(out, 0, byte(j-i))
			i = j
		} else {
			out = append(out, in[i])
			i++
		}
	}
	return out
}

// rleDecode inverses rleEncode. A trailing lone zero (no count byte) is
// preserved as a literal zero to match tdlib's tolerance.
func rleDecode(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); i++ {
		if in[i] == 0 {
			if i+1 < len(in) {
				n := int(in[i+1])
				for k := 0; k < n; k++ {
					out = append(out, 0)
				}
				i++
				continue
			}
			out = append(out, 0)
			continue
		}
		out = append(out, in[i])
	}
	return out
}
