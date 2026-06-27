package nbt

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

type NbtParser struct{}

func (p *NbtParser) ParseBinary(stream io.Reader, detectCompression bool) (NbtDocument, error) {
	if stream == nil {
		return NbtDocument{}, errors.New("stream cannot be nil")
	}

	prepared, err := p.PrepareStream(stream, detectCompression)
	if err != nil {
		return NbtDocument{}, err
	}

	reader := &NbtBinaryReader{stream: prepared}
	doc, err := reader.ReadDocument()
	if err != nil {
		return NbtDocument{}, err
	}

	return *doc, nil
}

func (p *NbtParser) ParseBinaryBytes(data []byte) (NbtDocument, error) {
	return p.ParseBinary(bytes.NewReader(data), true)
}

func (p *NbtParser) ParseSnbt(text string) (NbtDocument, error) {
	if strings.TrimSpace(text) == "" {
		return NbtDocument{}, errors.New("text cannot be empty")
	}

	parser := &snbtParser{text: text}
	tag, err := parser.parse()
	if err != nil {
		return NbtDocument{}, err
	}

	return *NewNbtDocument(tag), nil
}

func (p *NbtParser) PrepareStream(stream io.Reader, detectCompression bool) (io.Reader, error) {
	working, err := asReadSeeker(stream)
	if err != nil {
		return nil, err
	}

	if !detectCompression {
		_, err := working.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		return working, nil
	}

	header := make([]byte, 2)
	_, err = io.ReadFull(working, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}

	_, err = working.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	if header[0] == 0x1F && header[1] == 0x8B { // gzip magic number
		gzipReader, err := gzip.NewReader(working)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()

		var decompressed bytes.Buffer
		_, err = io.Copy(&decompressed, gzipReader)
		if err != nil {
			return nil, err
		}

		return bytes.NewReader(decompressed.Bytes()), nil
	}

	return working, nil
}

func asReadSeeker(stream io.Reader) (io.ReadSeeker, error) {
	if seeker, ok := stream.(io.ReadSeeker); ok {
		_, err := seeker.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		return seeker, nil
	}

	data, err := io.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(data), nil
}

type NbtBinaryReader struct {
	stream io.Reader
}

func (p *NbtBinaryReader) ReadDocument() (*NbtDocument, error) {
	typeByte, err := p.ReadByte()
	if err != nil {
		return nil, err
	}

	tagType := NbtTagType(typeByte)
	if tagType == NbtTagTypeEnd {
		return NewNbtDocument(NewNbtCompound(map[string]NbtTag{})), nil
	}

	_, err = p.ReadString()
	if err != nil {
		return nil, err
	}

	root, err := p.ReadTagPayload(tagType)
	if err != nil {
		return nil, err
	}

	return NewNbtDocument(root), nil
}

func (p *NbtBinaryReader) ReadTagPayload(tagType NbtTagType) (NbtTag, error) {
	switch tagType {
	case NbtTagTypeByte:
		value, err := p.ReadByte()
		if err != nil {
			return nil, err
		}
		return &NbtByte{Value: int8(value)}, nil
	case NbtTagTypeShort:
		value, err := p.ReadInt16()
		if err != nil {
			return nil, err
		}
		return &NbtShort{Value: value}, nil
	case NbtTagTypeInt:
		value, err := p.ReadInt32()
		if err != nil {
			return nil, err
		}
		return &NbtInt{Value: value}, nil
	case NbtTagTypeLong:
		value, err := p.ReadInt64()
		if err != nil {
			return nil, err
		}
		return &NbtLong{Value: value}, nil
	case NbtTagTypeFloat:
		value, err := p.ReadSingle()
		if err != nil {
			return nil, err
		}
		return &NbtFloat{Value: value}, nil
	case NbtTagTypeDouble:
		value, err := p.ReadDouble()
		if err != nil {
			return nil, err
		}
		return &NbtDouble{Value: value}, nil
	case NbtTagTypeByteArray:
		value, err := p.ReadByteArray()
		if err != nil {
			return nil, err
		}
		return &NbtByteArray{Values: value}, nil
	case NbtTagTypeString:
		value, err := p.ReadString()
		if err != nil {
			return nil, err
		}
		return &NbtString{Value: value}, nil
	case NbtTagTypeList:
		return p.ReadList()
	case NbtTagTypeCompound:
		return p.ReadCompound()
	case NbtTagTypeIntArray:
		value, err := p.ReadIntArray()
		if err != nil {
			return nil, err
		}
		return &NbtIntArray{Values: value}, nil
	case NbtTagTypeLongArray:
		value, err := p.ReadLongArray()
		if err != nil {
			return nil, err
		}
		return &NbtLongArray{Values: value}, nil
	case NbtTagTypeEnd:
		return NewNbtCompound(map[string]NbtTag{}), nil
	default:
		return nil, fmt.Errorf("unsupported NBT tag type '%d'", tagType)
	}
}

func (p *NbtBinaryReader) ReadCompound() (*NbtCompound, error) {
	items := make(map[string]NbtTag)
	for {
		typeByte, err := p.ReadByte()
		if err != nil {
			return nil, err
		}

		tagType := NbtTagType(typeByte)
		if tagType == NbtTagTypeEnd {
			break
		}

		name, err := p.ReadString()
		if err != nil {
			return nil, err
		}

		value, err := p.ReadTagPayload(tagType)
		if err != nil {
			return nil, err
		}

		items[name] = value
	}

	return NewNbtCompound(items), nil
}

func (p *NbtBinaryReader) ReadList() (*NbtList, error) {
	elementTypeByte, err := p.ReadByte()
	if err != nil {
		return nil, err
	}

	elementType := NbtTagType(elementTypeByte)
	length, err := p.ReadInt32()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, errors.New("encountered negative list length in NBT payload")
	}

	items := make([]NbtTag, 0, int(length))
	for i := int32(0); i < length; i++ {
		item, err := p.ReadTagPayload(elementType)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return NewNbtList(elementType, items), nil
}

func (p *NbtBinaryReader) ReadByteArray() ([]byte, error) {
	length, err := p.ReadInt32()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, errors.New("encountered negative byte array length in NBT payload")
	}

	buffer := make([]byte, int(length))
	err = readExactly(p.stream, buffer)
	if err != nil {
		return nil, err
	}

	return buffer, nil
}

func (p *NbtBinaryReader) ReadIntArray() ([]int32, error) {
	length, err := p.ReadInt32()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, errors.New("encountered negative int array length in NBT payload")
	}

	buffer := make([]int32, int(length))
	for i := 0; i < int(length); i++ {
		value, err := p.ReadInt32()
		if err != nil {
			return nil, err
		}
		buffer[i] = value
	}

	return buffer, nil
}

func (p *NbtBinaryReader) ReadLongArray() ([]int64, error) {
	length, err := p.ReadInt32()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, errors.New("encountered negative long array length in NBT payload")
	}

	buffer := make([]int64, int(length))
	for i := 0; i < int(length); i++ {
		value, err := p.ReadInt64()
		if err != nil {
			return nil, err
		}
		buffer[i] = value
	}

	return buffer, nil
}

func (p *NbtBinaryReader) ReadByte() (byte, error) {
	buffer := [1]byte{}
	err := readExactly(p.stream, buffer[:])
	if err != nil {
		return 0, err
	}
	return buffer[0], nil
}

func (p *NbtBinaryReader) ReadInt16() (int16, error) {
	buffer := [2]byte{}
	err := readExactly(p.stream, buffer[:])
	if err != nil {
		return 0, err
	}
	return int16(binary.BigEndian.Uint16(buffer[:])), nil
}

func (p *NbtBinaryReader) ReadInt32() (int32, error) {
	buffer := [4]byte{}
	err := readExactly(p.stream, buffer[:])
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(buffer[:])), nil
}

func (p *NbtBinaryReader) ReadInt64() (int64, error) {
	buffer := [8]byte{}
	err := readExactly(p.stream, buffer[:])
	if err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(buffer[:])), nil
}

func (p *NbtBinaryReader) ReadSingle() (float32, error) {
	buffer := [4]byte{}
	err := readExactly(p.stream, buffer[:])
	if err != nil {
		return 0, err
	}
	bits := binary.BigEndian.Uint32(buffer[:])
	return math.Float32frombits(bits), nil
}

func (p *NbtBinaryReader) ReadDouble() (float64, error) {
	buffer := [8]byte{}
	err := readExactly(p.stream, buffer[:])
	if err != nil {
		return 0, err
	}
	bits := binary.BigEndian.Uint64(buffer[:])
	return math.Float64frombits(bits), nil
}

func (p *NbtBinaryReader) ReadString() (string, error) {
	length, err := p.ReadInt16()
	if err != nil {
		return "", err
	}
	if length <= 0 {
		return "", nil
	}

	buffer := make([]byte, int(length))
	err = readExactly(p.stream, buffer)
	if err != nil {
		return "", err
	}

	return decodeMutf8(buffer)
}

func decodeMutf8(data []byte) (string, error) {
	chars := make([]rune, 0, len(data))

	for i := 0; i < len(data); i++ {
		b1 := data[i]

		if (b1 & 0x80) == 0 {
			chars = append(chars, rune(b1))
			continue
		}

		if (b1 & 0xE0) == 0xC0 {
			if i+1 >= len(data) {
				return "", errors.New("truncated MUTF-8 sequence")
			}
			b2 := data[i+1]
			i++
			if (b2 & 0xC0) != 0x80 {
				return "", errors.New("invalid MUTF-8 continuation byte")
			}

			codePoint := rune((b1&0x1F)<<6 | (b2 & 0x3F))
			chars = append(chars, codePoint)
			continue
		}

		if (b1 & 0xF0) == 0xE0 {
			if i+2 >= len(data) {
				return "", errors.New("truncated MUTF-8 sequence")
			}
			b2 := data[i+1]
			b3 := data[i+2]
			i += 2
			if (b2&0xC0) != 0x80 || (b3&0xC0) != 0x80 {
				return "", errors.New("invalid MUTF-8 continuation byte")
			}

			codePoint := rune((b1&0x0F)<<12 | (b2&0x3F)<<6 | (b3 & 0x3F))
			chars = append(chars, codePoint)
			continue
		}

		return "", fmt.Errorf("invalid MUTF-8 byte: 0x%X", b1)
	}

	return string(chars), nil
}

type snbtParser struct {
	text  string
	index int
}

func (p *snbtParser) parse() (NbtTag, error) {
	p.skipWhitespace()
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	p.skipWhitespace()
	if !p.isAtEnd() {
		return nil, errors.New("unexpected characters after SNBT payload")
	}

	return value, nil
}

func (p *snbtParser) parseValue() (NbtTag, error) {
	if p.match('{') {
		return p.parseCompound()
	}

	if p.match('[') {
		return p.parseListOrArray()
	}

	peek := p.peek()
	if peek == '"' || peek == '\'' {
		value, err := p.parseQuotedString()
		if err != nil {
			return nil, err
		}
		return &NbtString{Value: value}, nil
	}

	return p.parseScalar()
}

func (p *snbtParser) parseCompound() (NbtTag, error) {
	items := make(map[string]NbtTag)
	p.skipWhitespace()
	if p.match('}') {
		return NewNbtCompound(items), nil
	}

	for {
		p.skipWhitespace()
		key, err := p.parseKey()
		if err != nil {
			return nil, err
		}

		p.skipWhitespace()
		err = p.expect(':')
		if err != nil {
			return nil, err
		}

		p.skipWhitespace()
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		items[key] = value

		p.skipWhitespace()
		if p.match('}') {
			break
		}

		err = p.expect(',')
		if err != nil {
			return nil, err
		}
	}

	return NewNbtCompound(items), nil
}

func (p *snbtParser) parseListOrArray() (NbtTag, error) {
	p.skipWhitespace()
	if !p.isAtEnd() {
		designator := p.peek()
		next := p.lookAhead(1)
		if (designator == 'B' || designator == 'b' || designator == 'I' || designator == 'i' || designator == 'L' || designator == 'l') && next == ';' {
			typeRune := toUpperASCII(p.advance())
			err := p.expect(';')
			if err != nil {
				return nil, err
			}
			p.skipWhitespace()

			switch typeRune {
			case 'B':
				return p.parseByteArrayLiteral()
			case 'I':
				return p.parseIntArrayLiteral()
			case 'L':
				return p.parseLongArrayLiteral()
			default:
				return nil, errors.New("unsupported typed array designator in SNBT")
			}
		}
	}

	items := make([]NbtTag, 0)
	p.skipWhitespace()
	if p.match(']') {
		return NewNbtList(NbtTagTypeEnd, items), nil
	}

	for {
		p.skipWhitespace()
		item, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		if len(items) > 0 && item.Type() != items[0].Type() {
			return nil, errors.New("SNBT lists must contain elements of the same type")
		}

		items = append(items, item)
		p.skipWhitespace()
		if p.match(']') {
			break
		}

		err = p.expect(',')
		if err != nil {
			return nil, err
		}
	}

	elementType := NbtTagTypeEnd
	if len(items) > 0 {
		elementType = items[0].Type()
	}

	return NewNbtList(elementType, items), nil
}

func (p *snbtParser) parseByteArrayLiteral() (NbtTag, error) {
	values := make([]byte, 0)
	p.skipWhitespace()
	if p.match(']') {
		return &NbtByteArray{Values: values}, nil
	}

	for {
		tag, err := p.parseScalar()
		if err != nil {
			return nil, err
		}

		switch value := tag.(type) {
		case *NbtByte:
			values = append(values, byte(value.Value))
		case *NbtInt:
			values = append(values, byte(value.Value))
		case *NbtLong:
			values = append(values, byte(value.Value))
		default:
			return nil, errors.New("invalid element type for byte array")
		}

		p.skipWhitespace()
		if p.match(']') {
			break
		}

		err = p.expect(',')
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
	}

	return &NbtByteArray{Values: values}, nil
}

func (p *snbtParser) parseIntArrayLiteral() (NbtTag, error) {
	values := make([]int32, 0)
	p.skipWhitespace()
	if p.match(']') {
		return &NbtIntArray{Values: values}, nil
	}

	for {
		tag, err := p.parseScalar()
		if err != nil {
			return nil, err
		}

		switch value := tag.(type) {
		case *NbtInt:
			values = append(values, value.Value)
		case *NbtByte:
			values = append(values, int32(value.Value))
		case *NbtLong:
			if value.Value > math.MaxInt32 || value.Value < math.MinInt32 {
				return nil, errors.New("invalid element type for int array")
			}
			values = append(values, int32(value.Value))
		default:
			return nil, errors.New("invalid element type for int array")
		}

		p.skipWhitespace()
		if p.match(']') {
			break
		}

		err = p.expect(',')
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
	}

	return &NbtIntArray{Values: values}, nil
}

func (p *snbtParser) parseLongArrayLiteral() (NbtTag, error) {
	values := make([]int64, 0)
	p.skipWhitespace()
	if p.match(']') {
		return &NbtLongArray{Values: values}, nil
	}

	for {
		tag, err := p.parseScalar()
		if err != nil {
			return nil, err
		}

		switch value := tag.(type) {
		case *NbtLong:
			values = append(values, value.Value)
		case *NbtInt:
			values = append(values, int64(value.Value))
		case *NbtByte:
			values = append(values, int64(value.Value))
		default:
			return nil, errors.New("invalid element type for long array")
		}

		p.skipWhitespace()
		if p.match(']') {
			break
		}

		err = p.expect(',')
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
	}

	return &NbtLongArray{Values: values}, nil
}

func (p *snbtParser) parseQuotedString() (string, error) {
	quote := p.advance()
	var builder strings.Builder

	for !p.isAtEnd() {
		c := p.advance()
		if c == quote {
			return builder.String(), nil
		}

		if c == '\\' && !p.isAtEnd() {
			escape := p.advance()
			switch escape {
			case '"':
				builder.WriteByte('"')
			case '\'':
				builder.WriteByte('\'')
			case '\\':
				builder.WriteByte('\\')
			case 'n':
				builder.WriteByte('\n')
			case 'r':
				builder.WriteByte('\r')
			case 't':
				builder.WriteByte('\t')
			case '0':
				builder.WriteByte(0)
			default:
				builder.WriteByte(escape)
			}
			continue
		}

		builder.WriteByte(c)
	}

	return "", errors.New("unterminated string in SNBT payload")
}

func (p *snbtParser) parseKey() (string, error) {
	peek := p.peek()
	if peek == '"' || peek == '\'' {
		return p.parseQuotedString()
	}

	start := p.index
	for !p.isAtEnd() {
		c := p.peek()
		if isWhitespace(c) || c == ':' || c == '}' || c == ',' {
			break
		}
		p.advance()
	}

	if start == p.index {
		return "", errors.New("expected key in SNBT payload")
	}

	return p.text[start:p.index], nil
}

func (p *snbtParser) parseScalar() (NbtTag, error) {
	token := p.readToken()
	if token == "" {
		return nil, errors.New("unexpected empty token in SNBT payload")
	}

	if strings.EqualFold(token, "true") {
		return &NbtByte{Value: 1}, nil
	}
	if strings.EqualFold(token, "false") {
		return &NbtByte{Value: 0}, nil
	}

	suffix := byte(0)
	if len(token) > 0 {
		suffix = toLowerASCII(token[len(token)-1])
	}

	numberPart := token
	hasSuffix := suffix == 'b' || suffix == 's' || suffix == 'l' || suffix == 'f' || suffix == 'd'
	if hasSuffix && len(token) > 1 {
		numberPart = token[:len(token)-1]
	}

	if hasSuffix {
		switch suffix {
		case 'b':
			if value, err := strconv.ParseInt(numberPart, 10, 8); err == nil {
				return &NbtByte{Value: int8(value)}, nil
			}
		case 's':
			if value, err := strconv.ParseInt(numberPart, 10, 16); err == nil {
				return &NbtShort{Value: int16(value)}, nil
			}
		case 'l':
			if value, err := strconv.ParseInt(numberPart, 10, 64); err == nil {
				return &NbtLong{Value: value}, nil
			}
		case 'f':
			if value, err := strconv.ParseFloat(numberPart, 32); err == nil {
				return &NbtFloat{Value: float32(value)}, nil
			}
		case 'd':
			if value, err := strconv.ParseFloat(numberPart, 64); err == nil {
				return &NbtDouble{Value: value}, nil
			}
		}

		return &NbtString{Value: token}, nil
	}

	if strings.Contains(token, ".") || strings.ContainsAny(token, "eE") {
		if value, err := strconv.ParseFloat(token, 64); err == nil {
			return &NbtDouble{Value: value}, nil
		}
	}

	if value, err := strconv.ParseInt(token, 10, 32); err == nil {
		return &NbtInt{Value: int32(value)}, nil
	}

	if value, err := strconv.ParseInt(token, 10, 64); err == nil {
		return &NbtLong{Value: value}, nil
	}

	return &NbtString{Value: token}, nil
}

func (p *snbtParser) readToken() string {
	start := p.index
	for !p.isAtEnd() {
		c := p.peek()
		if isWhitespace(c) || c == ',' || c == ']' || c == '}' || c == ':' {
			break
		}
		p.advance()
	}

	return p.text[start:p.index]
}

func (p *snbtParser) match(expected byte) bool {
	if p.isAtEnd() || p.peek() != expected {
		return false
	}
	p.index++
	return true
}

func (p *snbtParser) expect(expected byte) error {
	if !p.match(expected) {
		return fmt.Errorf("expected '%c' in SNBT payload", expected)
	}
	return nil
}

func (p *snbtParser) peek() byte {
	if p.isAtEnd() {
		return 0
	}
	return p.text[p.index]
}

func (p *snbtParser) lookAhead(offset int) byte {
	position := p.index + offset
	if position >= len(p.text) {
		return 0
	}
	return p.text[position]
}

func (p *snbtParser) advance() byte {
	if p.isAtEnd() {
		return 0
	}
	ch := p.text[p.index]
	p.index++
	return ch
}

func (p *snbtParser) skipWhitespace() {
	for !p.isAtEnd() && isWhitespace(p.peek()) {
		p.index++
	}
}

func (p *snbtParser) isAtEnd() bool {
	return p.index >= len(p.text)
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func toUpperASCII(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

func toLowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

func readExactly(stream io.Reader, buffer []byte) error {
	_, err := io.ReadFull(stream, buffer)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return errors.New("unexpected end of stream while reading NBT payload")
		}
		return err
	}
	return nil
}
