package telemetry

import (
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// validUTF8 replaces invalid UTF-8 byte sequences with the Unicode
// replacement rune (U+FFFD). proto3 requires string fields to be valid UTF-8,
// so protojson.Marshal fails the entire OTLP export batch when any span, log,
// or metric carries invalid bytes. Dropping a record silently corrupts the
// audit surface; a record with replacement runes remains faithful evidence.
func validUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "�")
}

// sanitizeProtoStrings replaces invalid UTF-8 in every populated string field
// reachable from m. It is intentionally generic so all OTLP protojson writers
// share the same batch-preserving behavior.
func sanitizeProtoStrings(m proto.Message) {
	if m == nil {
		return
	}
	sanitizeMessageStrings(m.ProtoReflect())
}

func sanitizeMessageStrings(m protoreflect.Message) {
	if !m.IsValid() {
		return
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		sanitizeFieldString(m, fd, v)
		return true
	})
}

func sanitizeFieldString(m protoreflect.Message, fd protoreflect.FieldDescriptor, v protoreflect.Value) {
	if fd.IsMap() {
		sanitizeMapStrings(v.Map(), fd)
		return
	}
	if fd.IsList() {
		sanitizeListStrings(v.List(), fd)
		return
	}
	switch fd.Kind() {
	case protoreflect.StringKind:
		m.Set(fd, protoreflect.ValueOfString(validUTF8(v.String())))
	case protoreflect.MessageKind, protoreflect.GroupKind:
		sanitizeMessageStrings(v.Message())
	}
}

func sanitizeListStrings(list protoreflect.List, fd protoreflect.FieldDescriptor) {
	for i := 0; i < list.Len(); i++ {
		v := list.Get(i)
		switch fd.Kind() {
		case protoreflect.StringKind:
			list.Set(i, protoreflect.ValueOfString(validUTF8(v.String())))
		case protoreflect.MessageKind, protoreflect.GroupKind:
			sanitizeMessageStrings(v.Message())
		}
	}
}

func sanitizeMapStrings(m protoreflect.Map, fd protoreflect.FieldDescriptor) {
	valueDesc := fd.MapValue()
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		switch valueDesc.Kind() {
		case protoreflect.StringKind:
			m.Set(k, protoreflect.ValueOfString(validUTF8(v.String())))
		case protoreflect.MessageKind, protoreflect.GroupKind:
			sanitizeMessageStrings(v.Message())
		}
		return true
	})
}
