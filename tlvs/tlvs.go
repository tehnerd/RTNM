package tlvs

import(
    "bytes"
    "encoding/binary"
    "fmt"
)

type TLVHeader struct{
    TLV_type uint8
    TLV_subtype uint8
    TLV_length uint16
}

func (tlv TLVHeader)Encode() []byte {
    buffer := new(bytes.Buffer)
    err := binary.Write(buffer,binary.BigEndian,&tlv)
    if err != nil{
        fmt.Println(err)
    }
    return buffer.Bytes()
}


func (tlv *TLVHeader)Decode(buffer []byte){
    reader := bytes.NewReader(buffer)
    err := binary.Read(reader,binary.BigEndian,tlv)
    if err != nil{
        fmt.Println(err)
    }
}

//generate pb encapsulated in TLV(1,1)
func GeneratePBTLV(data []byte) []byte{
    tlv_header := TLVHeader{1,1,0}
    tlv_header.TLV_length = uint16(len(data)+4)
    msg := append(tlv_header.Encode(),data...)
    return msg
}

