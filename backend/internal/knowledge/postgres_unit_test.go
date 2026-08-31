package knowledge

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPostgresLexicalQueryNormalizesNaturalLanguageSafely(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "natural language", input: "What does the PostgreSQL retrieval approach use?", want: "what | does | the | postgresql | retrieval | approach | use"},
		{name: "deduplicates terms", input: "PostgreSQL retrieval PostgreSQL", want: "postgresql | retrieval"},
		{name: "removes tsquery operators", input: "alpha & beta | !gamma:delta", want: "alpha | beta | gamma | delta"},
		{name: "preserves unicode words", input: "  HỌC tiếng Việt 123 ", want: "học | tiếng | việt | 123"},
		{name: "punctuation only", input: "& | ! : ()", want: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := postgresLexicalQuery(testCase.input); got != testCase.want {
				t.Fatalf("postgresLexicalQuery(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
		})
	}
}

func TestPostgresPersistenceHelpersHandleEmptyAndMalformedValues(t *testing.T) {
	if positiveVersion(-1) != 1 || positiveVersion(0) != 1 || positiveVersion(3) != 3 {
		t.Fatal("positiveVersion did not normalize non-positive versions")
	}
	if vectorValue(nil) != nil {
		t.Fatal("vectorValue(nil) should remain SQL NULL")
	}
	if got, want := vectorValue([]float32{1, 0.25, -2}), "[1,0.25,-2]"; got != want {
		t.Fatalf("vectorValue() = %v, want %q", got, want)
	}
	valid := "[1, 0.25, -2]"
	parsed := parseVector(&valid)
	if len(parsed) != 3 || parsed[0] != 1 || parsed[1] != 0.25 || parsed[2] != -2 {
		t.Fatalf("parseVector(valid) = %#v", parsed)
	}
	if parseVector(nil) != nil {
		t.Fatal("parseVector(nil) should return nil")
	}
	empty := "[]"
	if parsed := parseVector(&empty); parsed != nil {
		t.Fatalf("parseVector(empty) = %#v, want nil", parsed)
	}
	malformed := "[not-a-vector]"
	if parsed := parseVector(&malformed); parsed != nil {
		t.Fatalf("parseVector(malformed) = %#v, want nil", parsed)
	}
	now := time.Now().UTC()
	if got := valueOrZero(&now); !got.Equal(now) {
		t.Fatalf("valueOrZero(time) = %v, want %v", got, now)
	}
	if got := valueOrZero(nil); !got.IsZero() {
		t.Fatalf("valueOrZero(nil) = %v, want zero", got)
	}
}

func TestPostgresRepositoryAndErrorMappingBoundaries(t *testing.T) {
	if _, err := NewPostgresRepository(nil); !errors.Is(err, ErrNilRepository) {
		t.Fatalf("NewPostgresRepository(nil) = %v, want ErrNilRepository", err)
	}
	if !errors.Is(mapKnowledgeError(pgx.ErrNoRows), ErrNotFound) {
		t.Fatal("pgx no-rows should map to ErrNotFound")
	}
	for _, testCase := range []struct {
		code string
		want error
	}{
		{code: "23505", want: ErrConflict},
		{code: "23503", want: ErrNotFound},
		{code: "55000", want: ErrRevisionImmutable},
	} {
		if !errors.Is(mapKnowledgeError(&pgconn.PgError{Code: testCase.code, Message: "sensitive provider detail"}), testCase.want) {
			t.Fatalf("PostgreSQL code %s did not map to %v", testCase.code, testCase.want)
		}
	}
	unknown := mapKnowledgeError(errors.New("sensitive provider detail"))
	if unknown == nil || unknown.Error() != "knowledge persistence failed" || strings.Contains(unknown.Error(), "sensitive") {
		t.Fatalf("unknown database error was not redacted: %v", unknown)
	}
	if mapKnowledgeError(nil) != nil {
		t.Fatal("nil database error should remain nil")
	}
}
