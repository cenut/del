package whisperv2
import (
	"bytes"
	"testing"
)
var filterTopicsCreationTests = []struct {
	topics [][]string
	filter [][][4]byte
}{
	{ 
		topics: [][]string{
			{"abc", "def", "ghi"},
			{"def"},
			{"ghi", "abc"},
		},
		filter: [][][4]byte{
			{{0x4e, 0x03, 0x65, 0x7a}, {0x34, 0x60, 0x7c, 0x9b}, {0x21, 0x41, 0x7d, 0xf9}},
			{{0x34, 0x60, 0x7c, 0x9b}},
			{{0x21, 0x41, 0x7d, 0xf9}, {0x4e, 0x03, 0x65, 0x7a}},
		},
	},
	{ 
		topics: [][]string{
			{"abc", "def", "ghi"},
			{},
			{""},
			{"def"},
		},
		filter: [][][4]byte{
			{{0x4e, 0x03, 0x65, 0x7a}, {0x34, 0x60, 0x7c, 0x9b}, {0x21, 0x41, 0x7d, 0xf9}},
			{},
			{},
			{{0x34, 0x60, 0x7c, 0x9b}},
		},
	},
}
var filterTopicsCreationFlatTests = []struct {
	topics []string
	filter [][][4]byte
}{
	{ 
		topics: []string{"abc", "def", "ghi"},
		filter: [][][4]byte{
			{{0x4e, 0x03, 0x65, 0x7a}},
			{{0x34, 0x60, 0x7c, 0x9b}},
			{{0x21, 0x41, 0x7d, 0xf9}},
		},
	},
	{ 
		topics: []string{"abc", "", "ghi"},
		filter: [][][4]byte{
			{{0x4e, 0x03, 0x65, 0x7a}},
			{},
			{{0x21, 0x41, 0x7d, 0xf9}},
		},
	},
}
func TestFilterTopicsCreation(t *testing.T) {
	for i, tt := range filterTopicsCreationTests {
		filter := NewFilterTopicsFromStrings(tt.topics...)
		if len(filter) != len(tt.topics) {
			t.Errorf("test %d: condition count mismatch: have %v, want %v", i, len(filter), len(tt.topics))
			continue
		}
		for j, condition := range filter {
			if len(condition) != len(tt.filter[j]) {
				t.Errorf("test %d, condition %d: size mismatch: have %v, want %v", i, j, len(condition), len(tt.filter[j]))
				continue
			}
			for k := 0; k < len(condition); k++ {
				if !bytes.Equal(condition[k][:], tt.filter[j][k][:]) {
					t.Errorf("test %d, condition %d, segment %d: filter mismatch: have 0x%x, want 0x%x", i, j, k, condition[k], tt.filter[j][k])
				}
			}
		}
		binary := make([][][]byte, len(tt.topics))
		for j, condition := range tt.topics {
			binary[j] = make([][]byte, len(condition))
			for k, segment := range condition {
				binary[j][k] = []byte(segment)
			}
		}
		filter = NewFilterTopics(binary...)
		if len(filter) != len(tt.topics) {
			t.Errorf("test %d: condition count mismatch: have %v, want %v", i, len(filter), len(tt.topics))
			continue
		}
		for j, condition := range filter {
			if len(condition) != len(tt.filter[j]) {
				t.Errorf("test %d, condition %d: size mismatch: have %v, want %v", i, j, len(condition), len(tt.filter[j]))
				continue
			}
			for k := 0; k < len(condition); k++ {
				if !bytes.Equal(condition[k][:], tt.filter[j][k][:]) {
					t.Errorf("test %d, condition %d, segment %d: filter mismatch: have 0x%x, want 0x%x", i, j, k, condition[k], tt.filter[j][k])
				}
			}
		}
	}
	for i, tt := range filterTopicsCreationFlatTests {
		filter := NewFilterTopicsFromStringsFlat(tt.topics...)
		if len(filter) != len(tt.topics) {
			t.Errorf("test %d: condition count mismatch: have %v, want %v", i, len(filter), len(tt.topics))
			continue
		}
		for j, condition := range filter {
			if len(condition) != len(tt.filter[j]) {
				t.Errorf("test %d, condition %d: size mismatch: have %v, want %v", i, j, len(condition), len(tt.filter[j]))
				continue
			}
			for k := 0; k < len(condition); k++ {
				if !bytes.Equal(condition[k][:], tt.filter[j][k][:]) {
					t.Errorf("test %d, condition %d, segment %d: filter mismatch: have 0x%x, want 0x%x", i, j, k, condition[k], tt.filter[j][k])
				}
			}
		}
		binary := make([][]byte, len(tt.topics))
		for j, topic := range tt.topics {
			binary[j] = []byte(topic)
		}
		filter = NewFilterTopicsFlat(binary...)
		if len(filter) != len(tt.topics) {
			t.Errorf("test %d: condition count mismatch: have %v, want %v", i, len(filter), len(tt.topics))
			continue
		}
		for j, condition := range filter {
			if len(condition) != len(tt.filter[j]) {
				t.Errorf("test %d, condition %d: size mismatch: have %v, want %v", i, j, len(condition), len(tt.filter[j]))
				continue
			}
			for k := 0; k < len(condition); k++ {
				if !bytes.Equal(condition[k][:], tt.filter[j][k][:]) {
					t.Errorf("test %d, condition %d, segment %d: filter mismatch: have 0x%x, want 0x%x", i, j, k, condition[k], tt.filter[j][k])
				}
			}
		}
	}
}
var filterCompareTests = []struct {
	matcher filterer
	message filterer
	match   bool
}{
	{ 
		matcher: filterer{to: "", from: "", matcher: newTopicMatcher()},
		message: filterer{to: "to", from: "from", matcher: newTopicMatcher(NewFilterTopicsFromStringsFlat("topic")...)},
		match:   true,
	},
	{ 
		matcher: filterer{to: "to", from: "", matcher: newTopicMatcher()},
		message: filterer{to: "to", from: "from", matcher: newTopicMatcher(NewFilterTopicsFromStringsFlat("topic")...)},
		match:   true,
	},
	{ 
		matcher: filterer{to: "to", from: "", matcher: newTopicMatcher()},
		message: filterer{to: "", from: "from", matcher: newTopicMatcher(NewFilterTopicsFromStringsFlat("topic")...)},
		match:   false,
	},
	{ 
		matcher: filterer{to: "", from: "from", matcher: newTopicMatcher()},
		message: filterer{to: "to", from: "from", matcher: newTopicMatcher(NewFilterTopicsFromStringsFlat("topic")...)},
		match:   true,
	},
	{ 
		matcher: filterer{to: "", from: "from", matcher: newTopicMatcher()},
		message: filterer{to: "to", from: "", matcher: newTopicMatcher(NewFilterTopicsFromStringsFlat("topic")...)},
		match:   false,
	},
	{ 
		matcher: filterer{to: "", from: "from", matcher: newTopicMatcher(NewFilterTopicsFromStringsFlat("topic")...)},
		message: filterer{to: "to", from: "from", matcher: newTopicMatcher(NewFilterTopicsFromStringsFlat("topic")...)},
		match:   true,
	},
	{ 
		matcher: filterer{to: "", from: "", matcher: newTopicMatcher(NewFilterTopicsFromStringsFlat("topic")...)},
		message: filterer{to: "to", from: "from", matcher: newTopicMatcher()},
		match:   false,
	},
}
func TestFilterCompare(t *testing.T) {
	for i, tt := range filterCompareTests {
		if match := tt.matcher.Compare(tt.message); match != tt.match {
			t.Errorf("test %d: match mismatch: have %v, want %v", i, match, tt.match)
		}
	}
}