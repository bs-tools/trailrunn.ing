package mcp

import (
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// camelCaseFieldMasks returns the arguments with the paths of their field
// masks in lowerCamelCase, the only form protojson reads: it refuses a path
// with an underscore. The API's comments, which agents read as the tools'
// documentation, name fields in snake_case, like "run.lactate_test", so
// agents send masks that way too.
func camelCaseFieldMasks(args json.RawMessage, md protoreflect.MessageDescriptor) json.RawMessage {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(args, &object); err != nil {
		// Not an object; protojson reports what's wrong with it.
		return args
	}
	changed := false
	fields := md.Fields()
	for key, value := range object {
		fd := fields.ByJSONName(key)
		if fd == nil {
			fd = fields.ByName(protoreflect.Name(key))
		}
		if fd == nil || fd.Message() == nil || fd.IsMap() {
			continue
		}
		var replaced json.RawMessage
		switch {
		case fd.Message().FullName() == "google.protobuf.FieldMask":
			var mask string
			if json.Unmarshal(value, &mask) != nil || !strings.Contains(mask, "_") {
				continue
			}
			replaced, _ = json.Marshal(camelCasePaths(mask))
		case fd.Message().FullName().Parent() == "google.protobuf":
			continue
		case fd.IsList():
			var items []json.RawMessage
			if json.Unmarshal(value, &items) != nil {
				continue
			}
			itemChanged := false
			for i, item := range items {
				if converted := camelCaseFieldMasks(item, fd.Message()); string(converted) != string(item) {
					items[i], itemChanged = converted, true
				}
			}
			if !itemChanged {
				continue
			}
			replaced, _ = json.Marshal(items)
		default:
			replaced = camelCaseFieldMasks(value, fd.Message())
			if string(replaced) == string(value) {
				continue
			}
		}
		object[key], changed = replaced, true
	}
	if !changed {
		return args
	}
	converted, err := json.Marshal(object)
	if err != nil {
		return args
	}
	return converted
}

// camelCasePaths converts the snake_case parts of a field mask's paths to
// lowerCamelCase, as protoc names fields in JSON: "run.lactate_test" becomes
// "run.lactateTest".
func camelCasePaths(mask string) string {
	var b strings.Builder
	upper := false
	for _, r := range mask {
		switch {
		case r == '_':
			upper = true
			continue
		case upper && r >= 'a' && r <= 'z':
			r -= 'a' - 'A'
		}
		upper = false
		b.WriteRune(r)
	}
	return b.String()
}
