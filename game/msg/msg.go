package msg //game

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/nickax/gofu/vec"
	"reflect"
)

type CanHandle interface {
	~string | int32 | uint32 | int16 | uint16 | float32 | float64 | byte | bool | *vec.V3 | []float32
}

var le = binary.LittleEndian

type MsgEnum byte

const (
	GameId     MsgEnum = 1
	Mesh       MsgEnum = 4
	Things     MsgEnum = 11
	Masses     MsgEnum = 12
	Highlit    MsgEnum = 13
	Camera     MsgEnum = 14
	Cursor     MsgEnum = 15
	Message    MsgEnum = 16
	Vectors    MsgEnum = 17
	PlayerIds  MsgEnum = 18
	CreateGame MsgEnum = 19
	JoinGame   MsgEnum = 20
	//	msgBoundValues      msgEnum = 21
	ValueChange       MsgEnum = 22
	Grid              MsgEnum = 23 //send the current grid position and axes to the client
	Save              MsgEnum = 24
	Load              MsgEnum = 25
	Mode              MsgEnum = 26 //send a message of the current (editor) mode - renders on client sceen
	Labels            MsgEnum = 27 //collection of player/telemetry labels
	LabelSet          MsgEnum = 28 //set of labels
	Clear             MsgEnum = 29 //clear all spheres/lines
	CentreOfMass      MsgEnum = 30 //centre of mass
	JoinAsController  MsgEnum = 32 //join as a controller (not a player)
	ControlToken      MsgEnum = 31 //control token (4 digit Pin) for display on the client (as a QR code)
	ControlPositions  MsgEnum = 33 //receive the control positions (tracked blobs)
	Sound             MsgEnum = 34 //sound message (play a sound)
	Detune            MsgEnum = 35 //detune (pitch) sound (user for rpm/revs/throttle/ engine sound)
	Movement          MsgEnum = 36 //moved masses
	BindValue         MsgEnum = 37
	ClearContextMenu  MsgEnum = 38 //clear the context menu (on the client)
	Telemetry         MsgEnum = 39 //send telemetry data (to the client)
	PositionInstances MsgEnum = 40 //send mesh instance positions (to the client) (trees etc)

	ConnectViewer   MsgEnum = 41 //(re)connects a viewer, an id of -1 will create a new viewer
	CreatePlayer    MsgEnum = 42 //create a new player (sign up)
	SignIn          MsgEnum = 46 //Sign in a player (to manage devices/choose games)
	AddPlayerToGame MsgEnum = 43 //Set a players game
	WatchPlayer     MsgEnum = 44 //Watch a player (set a viewers player)
	DeviceId        MsgEnum = 45 //sends a device id (and token) to the client

	Players MsgEnum = 47 //for serverside persistence/restore of players

)

// func (m MsgEnum) WriteTo(buff *bytes.Buffer) {
// 	binary.Write(buff, le, m)
// }

type Msg struct {
	MsgType MsgEnum
	Buff    *bytes.Buffer
}

// NewMsgFromBytes creates a Msg from a byte slice
// The type is loaded into the msgType and it's buffer ready to read further data
func NewFromBuff(buff *bytes.Buffer, expectedType MsgEnum) *Msg {

	msgType := byte(0)
	read(buff, &msgType)
	if msgType != byte(expectedType) {
		panic(fmt.Sprintf("Msg type mismatch: expected %v got %v", expectedType, msgType))
	}
	return &Msg{MsgType: MsgEnum(msgType), Buff: buff}
}

func readString(buff *bytes.Buffer) string {
	sl := uint16(0)
	binary.Read(buff, le, &sl)
	s := make([]byte, sl)
	binary.Read(buff, le, &s)
	return string(s)
}

func writeString(buff *bytes.Buffer, s string) {
	binary.Write(buff, le, uint16(len(s))) //write the length of the string
	binary.Write(buff, le, []byte(s))      //write the string

}

// func GenericRead[T CanHandle](buff *bytes.Buffer) T {
// 	var value T
// 	switch any(value).(type) {
// 	case string:
// 		value = any(readString(buff)).(T) //this is tricky - cast it to any, then typecast it to T
// 	case *vec.V3:
// 		value = any(readVec3(buff)).(T)

// 	case int32, uint32, int16, uint16, float32, float64, byte, bool, []float32:
// 		error := binary.Read(buff, le, &value)
// 		if error != nil {
// 			panic(error)
// 		}
// 	default:
// 		panic("unsupported type " + fmt.Sprintf("%T", value) + " in GenericRead")
// 	}
// 	return value
// }

func readVec3(buff *bytes.Buffer) *vec.V3 {
	p := vec.NewVec3(0, 0, 0)

	floats := make([]float32, 3) //the components of a vector are float64's but we downscale to 32bits for transmission/storage
	binary.Read(buff, le, &floats)
	p.X = float64(floats[0])
	p.Y = float64(floats[1])
	p.Z = float64(floats[2])

	return p
}

func writeVec3(buff *bytes.Buffer, v *vec.V3) {

	binary.Write(buff, le, float32(v.X))
	binary.Write(buff, le, float32(v.Y))
	binary.Write(buff, le, float32(v.Z))

}

func writeVec2(buff *bytes.Buffer, v *vec.V2) {

	binary.Write(buff, le, float32(v.X))
	binary.Write(buff, le, float32(v.Y))

}

func (m *Msg) Write(values ...any) {
	write(m.Buff, values...)
}

func write(buff *bytes.Buffer, values ...any) {
	for _, v := range values {
		switch any(v).(type) {
		case string:
			writeString(buff, any(v).(string))
		case vec.V3:
			writeVec3(buff, any(v).(*vec.V3))
		case vec.V2:
			writeVec2(buff, any(v).(*vec.V2))
		case int32, uint32, int16, uint16, float32, float64, byte, bool, []float32:
			error := binary.Write(buff, le, v)
			if error != nil {
				panic(error)
			}
		default:
			panic("unsupported type " + fmt.Sprintf("%T", v) + " in Write(Many)")
		}
	}
}

func (msg *Msg) Read(into ...any) {
	read(msg.Buff, into...)
}

// Read many values from the buffer into the provided pointers
func read(buff *bytes.Buffer, into ...any) {
	for i, v := range into {

		if reflect.TypeOf(v).Kind() != reflect.Ptr {
			panic("Read requires pointers")
		}

		switch any(v).(type) {
		case string:
			into[i] = readString(buff)
		case vec.V3:
			into[i] = readVec3(buff)

		case int32, uint32, int16, uint16, float32, float64, byte, bool, []float32:
			error := binary.Read(buff, le, into[i])
			if error != nil {
				panic(error)
			}
		default:
			panic("unsupported type " + fmt.Sprintf("%T", v) + " in Read(Many)")
		}
	}
}

// func GenericWrite[T CanHandle](buff *bytes.Buffer, v T) {

// 	switch any(v).(type) {
// 	case string:
// 		writeString(buff, any(v).(string))
// 	case vec.V3:
// 		writeVec3(buff, any(v).(*vec.V3))
// 	case int32, uint32, int16, uint16, float32, float64, byte, bool, []float32:
// 		binary.Write(buff, le, v)
// 		//buff.Write(any(v).([]byte))
// 	default:
// 		panic("unsupported type " + fmt.Sprintf("%T", v) + " in GenericWrite")
// 	}
// }

func Empty() *Msg {
	return &Msg{Buff: new(bytes.Buffer)}
}
func NewMsg(hdr MsgEnum, values ...any) *Msg {
	buff := new(bytes.Buffer)
	binary.Write(buff, le, byte(hdr)) //the type conversion is unnecessary but makes it clear we are writing a byte
	msg := &Msg{MsgType: hdr, Buff: buff}
	msg.Write(values...)
	return msg
}

func NewFromBytes(b []byte) *Msg {
	buff := bytes.NewBuffer(b)
	msgType := byte(0)
	binary.Read(buff, le, &msgType)
	return &Msg{MsgType: MsgEnum(msgType), Buff: buff}
}

// func (m *Msg) WriteByte(b byte) error {

// 	return binary.Write(m.Buff, le, b) //write byte
// }

// func (m *Msg) WriteFloat32s(slice []float32) error {
// 	return binary.Write(m.Buff, le, slice)
// }

// func (m *Msg) WriteUInt16s(slice []uint16) error {
// 	return binary.Write(m.Buff, le, slice)
// }

// func (m *Msg) WriteString(s string) error {
// 	binary.Write(m.Buff, le, uint16(len(s))) //write the length of the string
// 	binary.Write(m.Buff, le, []byte(s))      //write the string
// 	return nil
// }

// func (m *Msg) WriteFloat64(f float64) error {
// 	return binary.Write(m.Buff, le, &f) //write float64
// }

// // fancy dancy generic write function - but it's not clear if this is better than individual function - as you can't have a generic method
// func Write[T canHandle](m *Msg, v T) {
// 	binary.Write(m.Buff, le, &v)
// }

// func (m *Msg) WriteFloat32(f float32) {
// 	binary.Write(m.Buff, le, &f) //write float32
// }

// func (m *Msg) WriteInt32(i int32) {
// 	binary.Write(m.Buff, le, &i) //write int32
// }

// func (m *Msg) WriteUUint32(i uint32) {
// 	binary.Write(m.Buff, le, &i) //write int32
// }

// func (m *Msg) WriteUint16(i uint16) {
// 	binary.Write(m.Buff, le, &i) //write uint16
// }

// func (m *Msg) WriteInt16(i int16) {
// 	binary.Write(m.Buff, le, &i) //write int16
// }

// func (m *Msg) WriteUInt32(i uint32) {
// 	binary.Write(m.Buff, le, &i) //write int32
// }

// func (m *Msg) WriteBool(b bool) {
// 	var i byte
// 	if b {
// 		i = 1
// 	} else {
// 		i = 0
// 	}
// 	binary.Write(m.Buff, le, &i) //write byte
// }

// func (m *Msg) WriteVec3(v *vec.V3) {
// 	binary.Write(m.Buff, le, float32(v.X)) //NOTE you CANT write a Vec3 directly as it has float64 components
// 	binary.Write(m.Buff, le, float32(v.Y))
// 	binary.Write(m.Buff, le, float32(v.Z))
// }

// func (m *Msg) WriteVec2(v *vec.V2) {
// 	binary.Write(m.Buff, le, float32(v.X)) //NOTE you CANT write a Vec2 directly as it has float64 components
// 	binary.Write(m.Buff, le, float32(v.Y))
// }

func (m *Msg) AllBytes() []byte {
	return m.Buff.Bytes()
}

// func (m *Msg) ReadString() string {
// 	sl := uint16(0)
// 	binary.Read(m.Buff, le, &sl) //read the length of the string
// 	s := make([]byte, sl)
// 	binary.Read(m.Buff, le, &s)
// 	return string(s)
// }

// func (m *Msg) ReadUint32() uint32 {
// 	u := uint32(0)
// 	binary.Read(m.Buff, le, &u) //read the length of the string
// 	return u
// }

// func (m *Msg) ReadUint16() uint16 {
// 	u := uint16(0)
// 	binary.Read(m.Buff, le, &u) //read the length of the string
// 	return u
// }

// func (m *Msg) ReadUInt32() uint32 {
// 	var i uint32
// 	binary.Read(m.Buff, le, &i)
// 	return i
// }
