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
