package transform

import (
	"context"
	"errors"
	"testing"

	"github.com/carlodf/cetl/connector"
	"github.com/carlodf/cetl/opener"
)

type testCase struct {
	name           string
	sources        []opener.Opener
	opt            CSVDecoderOptions
	expectedRows   [][]string
	expectedHeader []string
	expectedErr    error
}

var cases = []testCase{
	{
		name: "infer header basic",
		sources: []opener.Opener{
			opener.InMemorySource{
				Data:       []byte("a,b\n1,2\n"),
				SourceName: "TestSource1",
			},
		},
		opt:            CSVDecoderOptions{Comma: ','},
		expectedRows:   [][]string{{"1", "2"}},
		expectedHeader: []string{"a", "b"},
	},
	{
		name: "infer header error",
		sources: []opener.Opener{
			opener.InMemorySource{
				Data:       []byte(""),
				SourceName: "TestSource1",
			},
		},
		opt:            CSVDecoderOptions{Comma: ','},
		expectedRows:   nil,
		expectedHeader: nil,
		expectedErr:    errors.New("unable to infer header from first record: EOF"),
	},
	{
		name: "valid explicit header",
		sources: []opener.Opener{
			opener.InMemorySource{
				Data:       []byte("1,2\n"),
				SourceName: "TestSource1",
			},
		},
		opt:            CSVDecoderOptions{Comma: ',', Header: []string{"a", "b"}},
		expectedRows:   [][]string{{"1", "2"}},
		expectedHeader: []string{"a", "b"},
	},
	{
		name: "explicit header fields count mismatch",
		sources: []opener.Opener{
			opener.InMemorySource{
				Data:       []byte("1,2,3\n"),
				SourceName: "TestSource1",
			},
		},
		opt:            CSVDecoderOptions{Comma: ',', Header: []string{"a", "b"}},
		expectedRows:   nil,
		expectedHeader: nil,
		expectedErr:    errors.New("record on line 1: wrong number of fields"),
	},
	{
		name: "header skip on new source",
		sources: []opener.Opener{
			opener.InMemorySource{
				Data:       []byte("col1,col2\na1,b1\n"),
				SourceName: "TestSource1",
			},
			opener.InMemorySource{
				Data:       []byte("col1,col2\na2,b2\n"),
				SourceName: "TestSource2",
			},
		},
		opt:            CSVDecoderOptions{Comma: ','},
		expectedRows:   [][]string{{"a1", "b1"}, {"a2", "b2"}},
		expectedHeader: []string{"col1", "col2"},
	},
	{
		name: "do no skip mismatching",
		sources: []opener.Opener{
			opener.InMemorySource{
				Data:       []byte("col1,col2\na1,b1\n"),
				SourceName: "TestSource1",
			},
			opener.InMemorySource{
				Data:       []byte("x,y\na2,b2\n"),
				SourceName: "TestSource2",
			},
		},
		opt:            CSVDecoderOptions{Comma: ','},
		expectedRows:   [][]string{{"a1", "b1"}, {"x", "y"}, {"a2", "b2"}},
		expectedHeader: []string{"col1", "col2"},
	},
	{
		name: "boundary condition empty source",
		sources: []opener.Opener{
			opener.InMemorySource{
				Data:       []byte("col1,col2\na1,b1\n"),
				SourceName: "TestSource1",
			},
			opener.InMemorySource{
				Data:       []byte(""),
				SourceName: "TestSource2",
			},
			opener.InMemorySource{
				Data:       []byte("a2,b2\n"),
				SourceName: "TestSource3",
			},
		},
		opt:            CSVDecoderOptions{Comma: ','},
		expectedRows:   [][]string{{"a1", "b1"}, {"a2", "b2"}},
		expectedHeader: []string{"col1", "col2"},
	},
	{
		name: "malformed header",
		sources: []opener.Opener{
			opener.InMemorySource{
				Data:       []byte("col1,col1\na1,b1\n"),
				SourceName: "TestSource1",
			},
		},
		opt:            CSVDecoderOptions{Comma: ','},
		expectedRows:   nil,
		expectedHeader: nil,
		expectedErr:    errors.New("malformed header: duplicate entry col1 in header [\"col1\" \"col1\"]"),
	},
}

func TestCSVDecoder(t *testing.T) {
	basicContext := context.Background()
	for _, tc := range cases {
		runTest(tc, t, basicContext)
	}
}

func runTest(tc testCase, t *testing.T, ctx context.Context) {
	decoder := NewCSVDecoder(tc.opt)
	it, DecoderErr := decoder.Decode(ctx, connector.NewMuxReader(ctx, tc.sources))
	if DecoderErr != nil && DecoderErr.Error() != tc.expectedErr.Error() {
		t.Fatalf("%q = expected error: %q, got: %q", tc.name, tc.expectedErr, DecoderErr)
	}
	for _, row := range tc.expectedRows {
		if !it.Next() {
			t.Fatalf("%q = expected row: %q but iterator does not have next", tc.name, row)
		}
		extractedRecord := it.Record()
		if !rowEqualRecord(row, extractedRecord) {
			t.Fatalf("%q = expected: %q but got %v", tc.name, row, extractedRecord)
		}
		if tc.expectedHeader != nil && !equalSilce(tc.expectedHeader, extractedRecord.Names()) {
			t.Fatalf("%q = expected header: %q but got: %q", tc.name, tc.expectedHeader, extractedRecord.Names())
		}
	}
	if tc.expectedRows == nil && DecoderErr == nil {
		if it.Next() {
			t.Fatalf("%q = expected no result but got: %q", tc.name, it.Record())
		}
		if it.Err().Error() != tc.expectedErr.Error() {
			t.Fatalf("%q = expected error: %q but got: %q", tc.name, tc.expectedErr, it.Err().Error())
		}
	}
}

func equalSilce(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func rowEqualRecord(a []string, b Extractor) bool {
	if len(a) != b.Len() {
		return false
	}
	for idx := range a {
		val, ok := b.ByIndex(idx)
		if !ok {
			return false
		}
		if a[idx] != val {
			return false
		}
	}
	return true
}
