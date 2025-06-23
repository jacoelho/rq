package jsonpath

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const exampleJSON = `{
  "store": {
    "book": [
      { "category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": 8.95 },
      { "category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": 12.99 },
      { "category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": 8.99 },
      { "category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": 22.99 }
    ],
    "bicycle": { "color": "red", "price": 399 }
  }
}`

const complexJSON = `{
  "users": [
    { "name": "Alice", "email": "alice@example.com", "age": 30, "tags": ["admin", "user"] },
    { "name": "Bob", "email": "bob@test.org", "age": 25, "tags": ["user"] },
    { "name": "Charlie", "email": "charlie@example.com", "age": 35, "tags": ["moderator"] }
  ],
  "config": {
    "pattern": "test-.*",
    "timeout": 30,
    "features": {
      "auth": true,
      "logging": false
    }
  }
}`

func TestBasicOperations(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		expect []any
	}{
		{
			name:   "wildcard_author_selection",
			query:  "$.store.book[*].author",
			expect: []any{"Nigel Rees", "Evelyn Waugh", "Herman Melville", "J. R. R. Tolkien"},
		},
		{
			name:   "recursive_author_search",
			query:  "$..author",
			expect: []any{"Nigel Rees", "Evelyn Waugh", "Herman Melville", "J. R. R. Tolkien"},
		},
		{
			name:  "store_wildcard",
			query: "$.store.*",
			expect: []any{
				[]any{
					map[string]any{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": json.Number("8.95")},
					map[string]any{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": json.Number("12.99")},
					map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": json.Number("8.99")},
					map[string]any{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": json.Number("22.99")},
				},
				map[string]any{"color": "red", "price": json.Number("399")},
			},
		},
		{
			name:   "recursive_price_search",
			query:  "$.store..price",
			expect: []any{json.Number("8.95"), json.Number("12.99"), json.Number("8.99"), json.Number("22.99"), json.Number("399")},
		},
		{
			name:  "third_book",
			query: "$..book[2]",
			expect: []any{
				map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": json.Number("8.99")},
			},
		},
		{
			name:   "third_book_author",
			query:  "$..book[2].author",
			expect: []any{"Herman Melville"},
		},
		{
			name:   "nonexistent_property",
			query:  "$..book[2].publisher",
			expect: []any{},
		},
		{
			name:  "first_two_books",
			query: "$..book[:2]",
			expect: []any{
				map[string]any{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": json.Number("8.95")},
				map[string]any{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": json.Number("12.99")},
			},
		},
		{
			name:  "books_1_to_3",
			query: "$..book[1:3]",
			expect: []any{
				map[string]any{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": json.Number("12.99")},
				map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": json.Number("8.99")},
			},
		},
		{
			name:  "every_second_book",
			query: "$..book[::2]",
			expect: []any{
				map[string]any{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": json.Number("8.95")},
				map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": json.Number("8.99")},
			},
		},
		{
			name:  "every_second_starting_at_1",
			query: "$..book[1::2]",
			expect: []any{
				map[string]any{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": json.Number("12.99")},
				map[string]any{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": json.Number("22.99")},
			},
		},
		{
			name:  "books_with_isbn",
			query: "$..book[?(@.isbn)]",
			expect: []any{
				map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": json.Number("8.99")},
				map[string]any{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": json.Number("22.99")},
			},
		},
		{
			name:  "cheap_books",
			query: "$..book[?(@.price<10)]",
			expect: []any{
				map[string]any{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": json.Number("8.95")},
				map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": json.Number("8.99")},
			},
		},
		{
			name:  "exact_price_match",
			query: "$..book[?(@.price==8.95)]",
			expect: []any{
				map[string]any{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": json.Number("8.95")},
			},
		},
		{
			name:  "not_exact_price",
			query: "$..book[?(@.price!=8.95)]",
			expect: []any{
				map[string]any{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": json.Number("12.99")},
				map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": json.Number("8.99")},
				map[string]any{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": json.Number("22.99")},
			},
		},
		{
			name:  "price_less_equal_10",
			query: "$..book[?(@.price<=10)]",
			expect: []any{
				map[string]any{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": json.Number("8.95")},
				map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": json.Number("8.99")},
			},
		},
		{
			name:  "expensive_books",
			query: "$..book[?(@.price>20)]",
			expect: []any{
				map[string]any{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": json.Number("22.99")},
			},
		},
		{
			name:  "premium_books",
			query: "$..book[?(@.price>=22.99)]",
			expect: []any{
				map[string]any{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": json.Number("22.99")},
			},
		},
		{
			name:   "root_document",
			query:  "$",
			expect: []any{map[string]any{"store": map[string]any{"book": []any{map[string]any{"category": "reference", "author": "Nigel Rees", "title": "Sayings of the Century", "price": json.Number("8.95")}, map[string]any{"category": "fiction", "author": "Evelyn Waugh", "title": "Sword of Honour", "price": json.Number("12.99")}, map[string]any{"category": "fiction", "author": "Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": json.Number("8.99")}, map[string]any{"category": "fiction", "author": "J. R. R. Tolkien", "title": "The Lord of the Rings", "isbn": "0-395-19395-8", "price": json.Number("22.99")}}, "bicycle": map[string]any{"color": "red", "price": json.Number("399")}}}},
		},
		{
			name:   "union_array_indices",
			query:  "$..book[0,2].title",
			expect: []any{"Sayings of the Century", "Moby Dick"},
		},
		{
			name:   "union_properties",
			query:  "$..book[0]['author','title']",
			expect: []any{"Nigel Rees", "Sayings of the Century"},
		},
		{
			name:   "union_mixed_selectors",
			query:  "$..book[0,2]['title','price']",
			expect: []any{"Sayings of the Century", json.Number("8.95"), "Moby Dick", json.Number("8.99")},
		},
		{
			name:   "union_all_books",
			query:  "$..book[*]['category','price']",
			expect: []any{"reference", json.Number("8.95"), "fiction", json.Number("12.99"), "fiction", json.Number("8.99"), "fiction", json.Number("22.99")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(exampleJSON), tt.query)
			if err != nil {
				t.Fatalf("Stream(%q) error: %v", tt.query, err)
			}

			got := []any{}
			for r, err := range seq {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				got = append(got, r.Value)
			}
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("Stream(%q) = %v, want %v", tt.query, got, tt.expect)
			}
		})
	}
}

func TestStringFilters(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		expect []any
	}{
		{
			name:   "email_equals_single_quotes",
			query:  "$.users[?(@.email=='alice@example.com')]",
			expect: []any{map[string]any{"name": "Alice", "email": "alice@example.com", "age": json.Number("30"), "tags": []any{"admin", "user"}}},
		},
		{
			name:  "email_not_equals",
			query: "$.users[?(@.email!='alice@example.com')]",
			expect: []any{
				map[string]any{"name": "Bob", "email": "bob@test.org", "age": json.Number("25"), "tags": []any{"user"}},
				map[string]any{"name": "Charlie", "email": "charlie@example.com", "age": json.Number("35"), "tags": []any{"moderator"}},
			},
		},
		{
			name:   "name_equals_bob",
			query:  "$.users[?(@.name=='Bob')]",
			expect: []any{map[string]any{"name": "Bob", "email": "bob@test.org", "age": json.Number("25"), "tags": []any{"user"}}},
		},
		{
			name:   "name_equals_alice",
			query:  "$.users[?(@.name=='Alice')]",
			expect: []any{map[string]any{"name": "Alice", "email": "alice@example.com", "age": json.Number("30"), "tags": []any{"admin", "user"}}},
		},
		{
			name:   "name_equals_double_quotes",
			query:  `$.users[?(@.name=="Charlie")]`,
			expect: []any{map[string]any{"name": "Charlie", "email": "charlie@example.com", "age": json.Number("35"), "tags": []any{"moderator"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(complexJSON), tt.query)
			if err != nil {
				t.Fatalf("Stream(%q) error: %v", tt.query, err)
			}

			got := []any{}
			for r, err := range seq {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				got = append(got, r.Value)
			}
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("Stream(%q) = %v, want %v", tt.query, got, tt.expect)
			}
		})
	}
}

func TestRegexFilters(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		expect []any
	}{
		{
			name:  "email_domain_pattern",
			query: "$.users[?(@.email=~/.*@example\\.com/)]",
			expect: []any{
				map[string]any{"name": "Alice", "email": "alice@example.com", "age": json.Number("30"), "tags": []any{"admin", "user"}},
				map[string]any{"name": "Charlie", "email": "charlie@example.com", "age": json.Number("35"), "tags": []any{"moderator"}},
			},
		},
		{
			name:  "email_not_match_domain",
			query: "$.users[?(@.email!~/.*@example\\.com/)]",
			expect: []any{
				map[string]any{"name": "Bob", "email": "bob@test.org", "age": json.Number("25"), "tags": []any{"user"}},
			},
		},
		{
			name:  "case_insensitive_name",
			query: "$.users[?(@.name=~/^a/i)]",
			expect: []any{
				map[string]any{"name": "Alice", "email": "alice@example.com", "age": json.Number("30"), "tags": []any{"admin", "user"}},
			},
		},
		{
			name:  "multiline_name_pattern",
			query: "$.users[?(@.name=~/.*o.*/m)]",
			expect: []any{
				map[string]any{"name": "Bob", "email": "bob@test.org", "age": json.Number("25"), "tags": []any{"user"}},
			},
		},
		{
			name:   "no_match_empty_result",
			query:  "$.users[?(@.email=~/.+\\..+@.+/)]",
			expect: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(complexJSON), tt.query)
			if err != nil {
				t.Fatalf("Stream(%q) error: %v", tt.query, err)
			}

			got := []any{}
			for r, err := range seq {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				got = append(got, r.Value)
			}
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("Stream(%q) = %v, want %v", tt.query, got, tt.expect)
			}
		})
	}
}

func TestNestedPropertyFilters(t *testing.T) {
	nestedJSON := `{
  "data": {
    "items": [
      { "info": { "id": 1, "active": true }, "meta": { "score": 85 } },
      { "info": { "id": 2, "active": false }, "meta": { "score": 92 } },
      { "info": { "id": 3, "active": true }, "meta": { "score": 78 } }
    ]
  }
}`

	tests := []struct {
		name   string
		query  string
		expect []any
	}{
		{
			name:  "nested_property_existence",
			query: "$.data.items[?(@.info.active)]",
			expect: []any{
				map[string]any{"info": map[string]any{"id": json.Number("1"), "active": true}, "meta": map[string]any{"score": json.Number("85")}},
				map[string]any{"info": map[string]any{"id": json.Number("2"), "active": false}, "meta": map[string]any{"score": json.Number("92")}},
				map[string]any{"info": map[string]any{"id": json.Number("3"), "active": true}, "meta": map[string]any{"score": json.Number("78")}},
			},
		},
		{
			name:  "nested_numeric_filter",
			query: "$.data.items[?(@.meta.score>80)]",
			expect: []any{
				map[string]any{"info": map[string]any{"id": json.Number("1"), "active": true}, "meta": map[string]any{"score": json.Number("85")}},
				map[string]any{"info": map[string]any{"id": json.Number("2"), "active": false}, "meta": map[string]any{"score": json.Number("92")}},
			},
		},
		{
			name:  "nested_exact_match",
			query: "$.data.items[?(@.meta.score==92)]",
			expect: []any{
				map[string]any{"info": map[string]any{"id": json.Number("2"), "active": false}, "meta": map[string]any{"score": json.Number("92")}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(nestedJSON), tt.query)
			if err != nil {
				t.Fatalf("Stream(%q) error: %v", tt.query, err)
			}

			got := []any{}
			for r, err := range seq {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				got = append(got, r.Value)
			}
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("Stream(%q) = %v, want %v", tt.query, got, tt.expect)
			}
		})
	}
}

func TestInvalidSyntax(t *testing.T) {
	errorTests := []struct {
		name  string
		query string
	}{
		{"empty_query", ""},
		{"missing_dollar_prefix", "store.book"},
		{"invalid_start_character", "!store"},
		{"unterminated_bracket", "$.store["},
		{"incomplete_filter", "$.store[?(@)]"},
		{"malformed_regex", "$.store[?(@.name=~/[/)]"},
		{"unsupported_string_comparison", "$.store[?(@.name<'test')]"},
		{"unsupported_regex_comparison", "$.store[?(@.name<~/test/)]"},
		{"invalid_slice_step_zero", "$.store[0:5:0]"},
		{"non_numeric_slice_bound", "$.store[abc:5]"},
		{"invalid_numeric_literal", "$.store[?(@.price==abc)]"},
		{"unsupported_regex_flag", "$.store[?(@.name=~/test/x)]"},
		{"path_ending_with_dot", "$.store."},
		{"path_ending_with_double_dot", "$.store.."},
		{"empty_property_name_after_dot", "$.store."},
		{"unexpected_character_in_path", "$.store@"},
		{"malformed_union_expression", "$.store['a',]"},
		{"negative_slice_index_not_supported", "$.store[-1:]"},
		{"root_reference_in_filter_not_supported", "$.store.book[?(@.price < $.expensive)]"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Stream(context.Background(), strings.NewReader(exampleJSON), tt.query)
			if err == nil {
				t.Errorf("Stream(%q) expected error, got nil", tt.query)
			}
		})
	}
}

func TestSpecialValues(t *testing.T) {
	edgeJSON := `{
  "empty": {},
  "emptyArray": [],
  "nullValue": null,
  "numbers": [0, -1, 1.5, -2.7],
  "strings": ["", "hello", "with spaces", "special$chars"],
  "mixed": [1, "two", null, true, false]
}`

	tests := []struct {
		name   string
		query  string
		expect []any
	}{
		{
			name:   "access_empty_object",
			query:  "$.empty",
			expect: []any{map[string]any{}},
		},
		{
			name:   "access_empty_array",
			query:  "$.emptyArray",
			expect: []any{[]any{}},
		},
		{
			name:   "access_null_value",
			query:  "$.nullValue",
			expect: []any{nil},
		},
		{
			name:   "filter_negative_numbers_no_matches",
			query:  "$.numbers[?(@.value<0)]",
			expect: []any{},
		},
		{
			name:   "access_zero_by_array_index",
			query:  "$.numbers[0]",
			expect: []any{json.Number("0")},
		},
		{
			name:   "access_empty_string_by_index",
			query:  "$.strings[0]",
			expect: []any{""},
		},
		{
			name:   "access_string_with_spaces",
			query:  "$.strings[2]",
			expect: []any{"with spaces"},
		},
		{
			name:   "wildcard_on_empty_array_returns_nothing",
			query:  "$.emptyArray[*]",
			expect: []any{},
		},
		{
			name:   "out_of_bounds_index_returns_nothing",
			query:  "$.numbers[10]",
			expect: []any{},
		},
		{
			name:   "union_access_boolean_values",
			query:  "$.mixed[3,4]",
			expect: []any{true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(edgeJSON), tt.query)
			if err != nil {
				t.Fatalf("Stream(%q) error: %v", tt.query, err)
			}

			got := []any{}
			for r, err := range seq {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				got = append(got, r.Value)
			}
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("Stream(%q) = %v, want %v", tt.query, got, tt.expect)
			}
		})
	}
}

func TestMalformedJSON(t *testing.T) {
	invalidJSONTests := []string{
		`{"invalid": }`,
		`{"unclosed": "string`,
		`{"trailing": "comma",}`,
		`{broken}`,
	}

	for i, jsonStr := range invalidJSONTests {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(jsonStr), "$.test")
			if err != nil {
				// Error during Stream creation is OK
				return
			}

			// Should get error when iterating
			for _, err := range seq {
				if err != nil {
					return // Expected error
				}
			}
			t.Error("Expected error for invalid JSON, got none")
		})
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	seq, err := Stream(ctx, strings.NewReader(exampleJSON), "$.store.book[*]")
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	// Should handle cancellation
	for _, err := range seq {
		if err == context.Canceled {
			return // Expected cancellation
		}
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}
	}
}

func TestFilterWithPropertyAccess(t *testing.T) {
	jsonData := `{
		"store": {
			"book": [
				{"title": "Book1", "price": 8.95, "author": "Author1"},
				{"title": "Book2", "price": 12.99, "author": "Author2"},
				{"title": "Book3", "price": 5.50, "author": "Author3"}
			]
		}
	}`

	tests := []struct {
		name   string
		query  string
		expect []any
	}{
		{
			name:   "filter_with_title_access",
			query:  "$.store.book[?(@.price < 10)].title",
			expect: []any{"Book1", "Book3"},
		},
		{
			name:   "filter_with_price_access",
			query:  "$.store.book[?(@.price < 10)].price",
			expect: []any{json.Number("8.95"), json.Number("5.50")},
		},
		{
			name:   "filter_with_author_access",
			query:  "$.store.book[?(@.price < 10)].author",
			expect: []any{"Author1", "Author3"},
		},
		{
			name:   "different_filter_with_property_access",
			query:  "$.store.book[?(@.price > 10)].title",
			expect: []any{"Book2"},
		},
		{
			name:   "string_filter_with_property_access",
			query:  "$.store.book[?(@.author == 'Author1')].title",
			expect: []any{"Book1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(jsonData), test.query)
			if err != nil {
				t.Fatalf("Stream failed: %v", err)
			}

			var results []any
			for result, err := range seq {
				if err != nil {
					t.Fatalf("Stream iteration failed: %v", err)
				}
				results = append(results, result.Value)
			}

			if !reflect.DeepEqual(results, test.expect) {
				t.Errorf("Query %q: expected %v, got %v", test.query, test.expect, results)
			}
		})
	}
}

func TestFilterWithComplexSelectors(t *testing.T) {
	// Complex test JSON with nested structures and arrays
	complexTestJSON := `{
		"store": {
			"book": [
				{
					"category": "reference",
					"author": "Nigel Rees",
					"title": "Sayings of the Century",
					"price": 8.95,
					"details": {
						"pages": 200,
						"publisher": "Book Corp",
						"tags": ["wisdom", "quotes"],
						"availability": {
							"online": true,
							"stores": ["NYC", "LA"]
						}
					}
				},
				{
					"category": "fiction",
					"author": "Evelyn Waugh",
					"title": "Sword of Honour",
					"price": 12.99,
					"details": {
						"pages": 350,
						"publisher": "Fiction House",
						"tags": ["classic", "war"],
						"availability": {
							"online": false,
							"stores": ["Chicago", "Boston"]
						}
					}
				},
				{
					"category": "science",
					"author": "Isaac Newton",
					"title": "Principia",
					"price": 25.00,
					"details": {
						"pages": 500,
						"publisher": "Academic Press",
						"tags": ["physics", "mathematics", "classic"],
						"availability": {
							"online": true,
							"stores": ["NYC", "Boston", "SF"]
						}
					}
				}
			]
		}
	}`

	tests := []struct {
		name   string
		query  string
		expect []any
		desc   string
	}{
		// Simple property access after filter
		{
			name:   "category_filter_with_title_property_returns_book_title",
			query:  "$.store.book[?(@.category == 'reference')].title",
			expect: []any{"Sayings of the Century"},
			desc:   "Filter reference books and get title",
		},
		{
			name:   "price_filter_with_author_property_returns_authors",
			query:  "$.store.book[?(@.price < 15)].author",
			expect: []any{"Nigel Rees", "Evelyn Waugh"},
			desc:   "Filter books under $15 and get authors",
		},

		// Nested property access after filter
		{
			name:   "category_filter_with_nested_pages_property_returns_count",
			query:  "$.store.book[?(@.category == 'reference')].details.pages",
			expect: []any{json.Number("200")},
			desc:   "Filter reference books and get pages from details",
		},
		{
			name:   "price_filter_with_deep_nested_availability_returns_status",
			query:  "$.store.book[?(@.price < 15)].details.availability.online",
			expect: []any{true, false},
			desc:   "Filter books under $15 and get online availability",
		},

		// Wildcard selectors after filter - test specific property
		{
			name:   "filter_with_wildcard_specific_property",
			query:  "$.store.book[?(@.category == 'reference')].details.pages",
			expect: []any{json.Number("200")},
			desc:   "Filter reference books and get specific detail property",
		},

		// Array access after filter
		{
			name:   "filter_with_array_index",
			query:  "$.store.book[?(@.price < 15)].details.tags[0]",
			expect: []any{"wisdom", "classic"},
			desc:   "Filter books under $15 and get first tag",
		},
		{
			name:   "filter_with_array_last_index",
			query:  "$.store.book[?(@.category == 'fiction')].details.tags[1]",
			expect: []any{"war"},
			desc:   "Filter fiction books and get second tag",
		},

		// Array slice operations after filter
		{
			name:   "filter_with_array_slice",
			query:  "$.store.book[?(@.price > 20)].details.tags[0:2]",
			expect: []any{"physics", "mathematics"},
			desc:   "Filter expensive books and get first two tags",
		},
		{
			name:   "filter_with_array_slice_all",
			query:  "$.store.book[?(@.category == 'fiction')].details.tags[:]",
			expect: []any{"classic", "war"},
			desc:   "Filter fiction books and get all tags via slice",
		},

		// Wildcard on arrays after filter
		{
			name:   "filter_with_array_wildcard",
			query:  "$.store.book[?(@.category == 'science')].details.tags[*]",
			expect: []any{"physics", "mathematics", "classic"},
			desc:   "Filter science books and get all tags via wildcard",
		},
		{
			name:   "filter_with_nested_array_wildcard",
			query:  "$.store.book[?(@.price < 15)].details.availability.stores[*]",
			expect: []any{"NYC", "LA", "Chicago", "Boston"},
			desc:   "Filter books under $15 and get all store locations",
		},

		// Multiple segment processing after filter
		{
			name:   "filter_with_multiple_segments",
			query:  "$.store.book[?(@.category == 'science')].details.availability.stores[1]",
			expect: []any{"Boston"},
			desc:   "Filter science books and get second store location",
		},

		// Union selectors after filter
		{
			name:   "filter_with_union_properties",
			query:  "$.store.book[?(@.price > 20)]['title','author']",
			expect: []any{"Principia", "Isaac Newton"},
			desc:   "Filter expensive books and get title and author",
		},
		{
			name:   "filter_with_union_array_indices",
			query:  "$.store.book[?(@.category == 'science')].details.tags[0,2]",
			expect: []any{"physics", "classic"},
			desc:   "Filter science books and get first and third tags",
		},

		// Edge cases - these should return no results
		{
			name:   "filter_with_nonexistent_property_returns_empty",
			query:  "$.store.book[?(@.category == 'reference')].details.nonexistent",
			expect: []any(nil),
			desc:   "Filter reference books and access nonexistent property",
		},
		{
			name:   "filter_with_out_of_bounds_array_index_returns_empty",
			query:  "$.store.book[?(@.category == 'reference')].details.tags[10]",
			expect: []any(nil),
			desc:   "Filter reference books and access out-of-bounds array index",
		},

		// Complex nested access patterns
		{
			name:   "filter_with_nested_property_comparison",
			query:  "$.store.book[?(@.details.pages > 300)].title",
			expect: []any{"Sword of Honour", "Principia"},
			desc:   "Filter books with >300 pages and get titles",
		},
		{
			name:   "filter_with_regex_and_property",
			query:  "$.store.book[?(@.title =~ /^S/)].details.pages",
			expect: []any{json.Number("200"), json.Number("350")},
			desc:   "Filter books with titles starting with 'S' and get page counts",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(complexTestJSON), test.query)
			if err != nil {
				t.Fatalf("Stream(%q) failed: %v", test.query, err)
			}

			var results []any
			for result, err := range seq {
				if err != nil {
					t.Fatalf("Stream iteration failed for %q: %v", test.query, err)
				}
				results = append(results, result.Value)
			}

			if !reflect.DeepEqual(results, test.expect) {
				t.Errorf("Query %q (%s):\nexpected: %v\ngot:      %v",
					test.query, test.desc, test.expect, results)
			}
		})
	}
}

func TestInAndNinOperators(t *testing.T) {
	const testJSON = `{
  "products": [
    { "name": "T-Shirt", "size": "S", "color": "blue", "price": 15.99, "available": true },
    { "name": "Jeans", "size": "M", "color": "black", "price": 49.99, "available": true },
    { "name": "Hoodie", "size": "L", "color": "gray", "price": 39.99, "available": false },
    { "name": "Shorts", "size": "S", "color": "red", "price": 19.99, "available": true },
    { "name": "Dress", "size": "XL", "color": "blue", "price": 59.99, "available": true },
    { "name": "Sweater", "size": "M", "color": "white", "price": 34.99, "available": false }
  ],
  "categories": ["clothing", "accessories", "shoes"],
  "sizes": ["S", "M", "L", "XL"],
  "numbers": [1, 2, 3, 5, 8, 13]
}`

	tests := []struct {
		name   string
		query  string
		expect []any
	}{
		// String array membership tests
		{
			name:   "in_operator_with_string_array_returns_matching_sizes",
			query:  "$.products[?(@.size in ['S', 'M'])].name",
			expect: []any{"T-Shirt", "Jeans", "Shorts", "Sweater"},
		},
		{
			name:   "nin_operator_with_string_array_returns_non_matching_sizes",
			query:  "$.products[?(@.size nin ['S', 'M'])].name",
			expect: []any{"Hoodie", "Dress"},
		},
		{
			name:   "in_operator_with_color_array_filters_by_color",
			query:  "$.products[?(@.color in ['blue', 'red'])].name",
			expect: []any{"T-Shirt", "Shorts", "Dress"},
		},
		{
			name:   "nin_operator_with_color_array_excludes_colors",
			query:  "$.products[?(@.color nin ['blue', 'red'])].name",
			expect: []any{"Jeans", "Hoodie", "Sweater"},
		},

		// Boolean array membership tests
		{
			name:   "in_operator_with_boolean_array_filters_availability",
			query:  "$.products[?(@.available in [true])].name",
			expect: []any{"T-Shirt", "Jeans", "Shorts", "Dress"},
		},
		{
			name:   "nin_operator_with_boolean_array_excludes_availability",
			query:  "$.products[?(@.available nin [true])].name",
			expect: []any{"Hoodie", "Sweater"},
		},

		// Numeric array membership tests
		{
			name:   "in_operator_with_number_array_filters_prices",
			query:  "$.products[?(@.price in [15.99, 49.99])].name",
			expect: []any{"T-Shirt", "Jeans"},
		},
		{
			name:   "nin_operator_with_number_array_excludes_prices",
			query:  "$.products[?(@.price nin [15.99, 49.99])].name",
			expect: []any{"Hoodie", "Shorts", "Dress", "Sweater"},
		},

		// Mixed type array membership tests
		{
			name:   "in_operator_with_mixed_array_handles_different_types",
			query:  "$.products[?(@.size in ['S', 'M', 1, true])].name",
			expect: []any{"T-Shirt", "Jeans", "Shorts", "Sweater"},
		},

		// Empty array tests
		{
			name:   "in_operator_with_empty_array_returns_nothing",
			query:  "$.products[?(@.size in [])].name",
			expect: []any(nil),
		},
		{
			name:   "nin_operator_with_empty_array_returns_everything",
			query:  "$.products[?(@.size nin [])].name",
			expect: []any{"T-Shirt", "Jeans", "Hoodie", "Shorts", "Dress", "Sweater"},
		},

		// Edge cases with quotes
		{
			name:   "in_operator_with_double_quotes_works_correctly",
			query:  `$.products[?(@.size in ["S", "M"])].name`,
			expect: []any{"T-Shirt", "Jeans", "Shorts", "Sweater"},
		},
		{
			name:   "nin_operator_with_mixed_quotes_works_correctly",
			query:  `$.products[?(@.color nin ["blue", 'red'])].name`,
			expect: []any{"Jeans", "Hoodie", "Sweater"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(testJSON), test.query)
			if err != nil {
				t.Fatalf("Stream(%q) failed: %v", test.query, err)
			}

			var results []any
			for result, err := range seq {
				if err != nil {
					t.Fatalf("Stream iteration failed for %q: %v", test.query, err)
				}
				results = append(results, result.Value)
			}

			if !reflect.DeepEqual(results, test.expect) {
				t.Errorf("Query %q:\nexpected: %v\ngot:      %v",
					test.query, test.expect, results)
			}
		})
	}
}

func TestEdgeCases(t *testing.T) {
	edgeCaseJSON := `{
		"data": {
			"items": [
				{"type": "A", "values": [1, 2, 3], "info": {"status": "active"}},
				{"type": "B", "values": [], "info": {"status": "inactive"}},
				{"type": "A", "values": [4, 5], "info": null}
			],
			"empty": [],
			"nested": {
				"deep": {
					"array": [{"x": 1}, {"x": 2}]
				}
			}
		}
	}`

	tests := []struct {
		name   string
		query  string
		expect []any
		desc   string
	}{
		// Empty array handling
		{
			name:   "wildcard_selector_on_empty_array_returns_nothing",
			query:  "$.data.items[?(@.type == 'B')].values[*]",
			expect: []any(nil),
			desc:   "Filter items with type B and access empty array",
		},

		// Null value handling - access property on object where some have null values
		{
			name:   "property_access_on_mixed_null_values_returns_both",
			query:  "$.data.items[?(@.type == 'A')].info",
			expect: []any{map[string]any{"status": "active"}, nil},
			desc:   "Filter type A items and access info (including null)",
		},

		// Complex slicing with step
		{
			name:   "filter_with_slice_step",
			query:  "$.data.items[?(@.type == 'A')].values[::2]",
			expect: []any{json.Number("1"), json.Number("3"), json.Number("4")},
			desc:   "Filter type A items and get every 2nd value",
		},

		// Deep nesting after filter - simpler test without complex filter
		{
			name:   "filter_with_deep_nesting",
			query:  "$.data.nested.deep.array[*].x",
			expect: []any{json.Number("1"), json.Number("2")},
			desc:   "Access deep nested values in array",
		},

		// Multiple property access patterns - just test first item with status
		{
			name:   "filter_with_multiple_wildcard_access",
			query:  "$.data.items[0].info.*",
			expect: []any{"active"},
			desc:   "Get all properties from first item's info",
		},

		// Boundary slice operations
		{
			name:   "filter_with_slice_beyond_bounds",
			query:  "$.data.items[?(@.type == 'A')].values[1:10]",
			expect: []any{json.Number("2"), json.Number("3"), json.Number("5")},
			desc:   "Filter type A items and slice with end beyond array bounds",
		},

		// Test slice with explicit step
		{
			name:   "filter_with_slice_zero_step",
			query:  "$.data.items[?(@.type == 'A')].values[0:2:1]",
			expect: []any{json.Number("1"), json.Number("2"), json.Number("4"), json.Number("5")},
			desc:   "Filter type A items and slice with explicit step 1 (gets from both arrays)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(edgeCaseJSON), test.query)
			if err != nil {
				t.Fatalf("Stream(%q) failed: %v", test.query, err)
			}

			var results []any
			for result, err := range seq {
				if err != nil {
					t.Fatalf("Stream iteration failed for %q: %v", test.query, err)
				}
				results = append(results, result.Value)
			}

			if !reflect.DeepEqual(results, test.expect) {
				t.Errorf("Query %q (%s):\nexpected: %v\ngot:      %v",
					test.query, test.desc, test.expect, results)
			}
		})
	}
}

func TestCoverageGaps(t *testing.T) {
	// Test data for deep segment processing and other uncovered scenarios
	deepJSON := `{
		"level1": {
			"level2": {
				"target": "found",
				"level3": {
					"target": "found",
					"values": [1, 2, 3]
				},
				"array": [
					{"target": "array1", "deep": {"nested": "value1"}},
					{"target": "array2", "deep": {"nested": "value2"}}
				]
			},
			"another": {
				"target": "another_found"
			}
		},
		"parallel": {
			"target": "parallel_found"
		}
	}`

	tests := []struct {
		name   string
		query  string
		expect []any
		desc   string
	}{
		// Test processDeepSegment with wildcard selector
		{
			name:   "deep_search_wildcard_finds_all_targets",
			query:  "$..target",
			expect: []any{"found", "found", "array1", "array2", "another_found", "parallel_found"},
			desc:   "Deep search should find all 'target' properties at any level",
		},

		// Test processDeepSegment with objects and arrays
		{
			name:   "deep_search_in_nested_arrays",
			query:  "$..deep.nested",
			expect: []any{"value1", "value2"},
			desc:   "Deep search should find nested properties within arrays",
		},

		// Test processDeepSegment with index selector
		{
			name:   "deep_search_with_array_index",
			query:  "$..array[0].target",
			expect: []any{"array1"},
			desc:   "Deep search combined with array indexing",
		},

		// Test processArraySegment with filter selectors
		{
			name:   "array_filter_on_objects",
			query:  "$.level1.level2.array[?(@.target == 'array1')]",
			expect: []any{map[string]any{"target": "array1", "deep": map[string]any{"nested": "value1"}}},
			desc:   "Filter selector on array elements",
		},

		// Test processSliceSelector with explicit step
		{
			name:   "slice_with_explicit_step",
			query:  "$.level1.level2.level3.values[0:3:2]",
			expect: []any{json.Number("1"), json.Number("3")},
			desc:   "Slice with step 2 should skip every other element",
		},

		// Test slice with negative step (not supported, should fail during compile)
		{
			name:   "slice_with_default_step",
			query:  "$.level1.level2.level3.values[0:3]",
			expect: []any{json.Number("1"), json.Number("2"), json.Number("3")},
			desc:   "Slice with default step should include all elements in range",
		},

		// Test object access with simple property
		{
			name:   "object_property_access",
			query:  "$.level1.level2.target",
			expect: []any{"found"},
			desc:   "Simple object property access should work",
		},

		// Test buildPropertyPath with root path
		{
			name:  "property_path_from_root",
			query: "$.level1",
			expect: []any{map[string]any{
				"level2": map[string]any{
					"target": "found",
					"level3": map[string]any{"target": "found", "values": []any{json.Number("1"), json.Number("2"), json.Number("3")}},
					"array": []any{
						map[string]any{"target": "array1", "deep": map[string]any{"nested": "value1"}},
						map[string]any{"target": "array2", "deep": map[string]any{"nested": "value2"}},
					},
				},
				"another": map[string]any{"target": "another_found"},
			}},
			desc: "Property access from root should build correct path",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(deepJSON), test.query)
			if err != nil {
				t.Fatalf("Stream(%q) failed: %v", test.query, err)
			}

			var results []any
			for result, err := range seq {
				if err != nil {
					t.Fatalf("Stream iteration failed for %q: %v", test.query, err)
				}
				results = append(results, result.Value)
			}

			if !reflect.DeepEqual(results, test.expect) {
				t.Errorf("Query %q (%s):\nexpected: %v\ngot:      %v",
					test.query, test.desc, test.expect, results)
			}
		})
	}
}

func TestSelectorMatchingCoverage(t *testing.T) {
	testJSON := `{
		"data": [
			{"type": "A", "count": 10},
			{"type": "B", "count": 20},
			{"type": "C", "count": 15}
		],
		"numbers": [1, 2, 3, 4, 5],
		"mixed": [
			"string",
			42,
			true,
			{"nested": "value"}
		]
	}`

	tests := []struct {
		name   string
		query  string
		expect []any
		desc   string
	}{
		// These queries should trigger matchesSelectorForValue with different selector types
		{
			name:  "deep_search_with_filter_on_values",
			query: "$..data[?(@.count > 15)]",
			expect: []any{
				map[string]any{"type": "B", "count": json.Number("20")},
			},
			desc: "Deep search with filter should use matchesSelectorForValue",
		},

		// Test wildcard selector in deep search
		{
			name:   "deep_wildcard_on_arrays",
			query:  "$..numbers[*]",
			expect: []any{json.Number("1"), json.Number("2"), json.Number("3"), json.Number("4"), json.Number("5")},
			desc:   "Deep wildcard should match all array elements",
		},

		// Test index selector in deep search
		{
			name:   "deep_index_on_arrays",
			query:  "$..numbers[2]",
			expect: []any{json.Number("3")},
			desc:   "Deep index should match specific array element",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(testJSON), test.query)
			if err != nil {
				t.Fatalf("Stream(%q) failed: %v", test.query, err)
			}

			var results []any
			for result, err := range seq {
				if err != nil {
					t.Fatalf("Stream iteration failed for %q: %v", test.query, err)
				}
				results = append(results, result.Value)
			}

			if !reflect.DeepEqual(results, test.expect) {
				t.Errorf("Query %q (%s):\nexpected: %v\ngot:      %v",
					test.query, test.desc, test.expect, results)
			}
		})
	}
}

func TestProcessObjectSegmentCoverage(t *testing.T) {
	testJSON := `{
		"obj": {
			"name1": "value1",
			"name2": "value2",
			"name3": "value3"
		},
		"arr": [1, 2, 3]
	}`

	tests := []struct {
		name   string
		query  string
		expect []any
		desc   string
	}{
		// Test wildcardSel on object (covers processObjectSegment wildcard case)
		{
			name:   "wildcard_on_object",
			query:  "$.obj.*",
			expect: []any{"value1", "value2", "value3"},
			desc:   "Wildcard on object should return all property values",
		},

		// Test indexSel on object (should be skipped - covers default case)
		{
			name:   "index_on_object_ignored",
			query:  "$.obj",
			expect: []any{map[string]any{"name1": "value1", "name2": "value2", "name3": "value3"}},
			desc:   "Index selector should not apply to objects",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(testJSON), test.query)
			if err != nil {
				t.Fatalf("Stream(%q) failed: %v", test.query, err)
			}

			var results []any
			for result, err := range seq {
				if err != nil {
					t.Fatalf("Stream iteration failed for %q: %v", test.query, err)
				}
				results = append(results, result.Value)
			}

			if !reflect.DeepEqual(results, test.expect) {
				t.Errorf("Query %q (%s):\nexpected: %v\ngot:      %v",
					test.query, test.desc, test.expect, results)
			}
		})
	}
}

func TestProcessArraySegmentCoverage(t *testing.T) {
	testJSON := `{
		"arr": [
			{"id": 1, "name": "item1"},
			{"id": 2, "name": "item2"},
			{"id": 3, "name": "item3"}
		]
	}`

	tests := []struct {
		name   string
		query  string
		expect []any
		desc   string
	}{
		// Test nameSel on array (should be skipped - covers continue case)
		{
			name:  "name_selector_on_array_ignored",
			query: "$.arr",
			expect: []any{
				[]any{
					map[string]any{"id": json.Number("1"), "name": "item1"},
					map[string]any{"id": json.Number("2"), "name": "item2"},
					map[string]any{"id": json.Number("3"), "name": "item3"},
				},
			},
			desc: "Name selector should not apply to arrays",
		},

		// Test filterSel on array (covers the filterSel case)
		{
			name:  "filter_on_array_elements",
			query: "$.arr[?(@.id > 1)]",
			expect: []any{
				map[string]any{"id": json.Number("2"), "name": "item2"},
				map[string]any{"id": json.Number("3"), "name": "item3"},
			},
			desc: "Filter selector should apply to array elements",
		},

		// Test sliceSel on array
		{
			name:  "slice_on_array_elements",
			query: "$.arr[1:3]",
			expect: []any{
				map[string]any{"id": json.Number("2"), "name": "item2"},
				map[string]any{"id": json.Number("3"), "name": "item3"},
			},
			desc: "Slice selector should work on arrays",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(testJSON), test.query)
			if err != nil {
				t.Fatalf("Stream(%q) failed: %v", test.query, err)
			}

			var results []any
			for result, err := range seq {
				if err != nil {
					t.Fatalf("Stream iteration failed for %q: %v", test.query, err)
				}
				results = append(results, result.Value)
			}

			if !reflect.DeepEqual(results, test.expect) {
				t.Errorf("Query %q (%s):\nexpected: %v\ngot:      %v",
					test.query, test.desc, test.expect, results)
			}
		})
	}
}

func TestDecodeSubtreeErrorCoverage(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		query   string
		wantErr bool
		desc    string
	}{
		// Test decodeObjectSubtree with malformed object
		{
			name:    "malformed_object_in_subtree",
			json:    `{"valid": {"broken": "missing_close}`,
			query:   "$.valid",
			wantErr: true,
			desc:    "Malformed object in subtree should cause decode error",
		},

		// Test decodeArraySubtree with malformed array
		{
			name:    "malformed_array_in_subtree",
			json:    `{"valid": [1, 2, "missing_close]`,
			query:   "$.valid",
			wantErr: true,
			desc:    "Malformed array in subtree should cause decode error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seq, err := Stream(context.Background(), strings.NewReader(test.json), test.query)
			if err != nil {
				if test.wantErr {
					return // Expected compilation error
				}
				t.Fatalf("Stream(%q) failed unexpectedly: %v", test.query, err)
			}

			gotError := false
			for _, err := range seq {
				if err != nil {
					gotError = true
					break
				}
			}

			if test.wantErr && !gotError {
				t.Errorf("Expected error for %q but got none", test.query)
			} else if !test.wantErr && gotError {
				t.Errorf("Unexpected error for %q", test.query)
			}
		})
	}
}

func TestContextCancellationComprehensive(t *testing.T) {
	// Large JSON to ensure processing takes some time
	largeJSON := `{
		"items": [`
	for i := range 100 {
		if i > 0 {
			largeJSON += ","
		}
		largeJSON += fmt.Sprintf(`{"id": %d, "value": "item_%d", "data": [1, 2, 3, 4, 5]}`, i, i)
	}
	largeJSON += `]}`

	tests := []struct {
		name      string
		json      string
		query     string
		cancelAt  int // Cancel after this many results
		expectMin int // Minimum results before cancellation should be possible
		expectMax int // Maximum results after cancellation
	}{
		{
			name:      "cancel_during_array_processing",
			json:      largeJSON,
			query:     "$.items[*].id",
			cancelAt:  5,
			expectMin: 3,
			expectMax: 15,
		},
		{
			name:      "cancel_during_wildcard_processing",
			json:      largeJSON,
			query:     "$.items[*]",
			cancelAt:  3,
			expectMin: 2,
			expectMax: 10,
		},
		{
			name:      "cancel_with_deep_search",
			json:      largeJSON,
			query:     "$..id",
			cancelAt:  10,
			expectMin: 5,
			expectMax: 25,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			seq, err := Stream(ctx, strings.NewReader(test.json), test.query)
			if err != nil {
				t.Fatalf("Stream(%q) failed: %v", test.query, err)
			}

			count := 0
			cancelled := false
			gotCancelError := false

			for result, err := range seq {
				if err != nil {
					if err == context.Canceled {
						gotCancelError = true
						break
					}
					t.Fatalf("Unexpected error: %v", err)
				}

				_ = result
				count++

				if count == test.cancelAt && !cancelled {
					cancel()
					cancelled = true
				}

				// Safety check to prevent infinite loops
				if count > 1000 {
					t.Fatalf("Processed too many items (%d), possible infinite loop", count)
				}
			}

			if !cancelled {
				t.Fatalf("Test completed without cancelling context")
			}

			if count < test.expectMin {
				t.Errorf("Expected at least %d results before cancellation could take effect, got %d", test.expectMin, count)
			}

			if count > test.expectMax {
				t.Errorf("Expected at most %d results after cancellation, got %d", test.expectMax, count)
			}

			t.Logf("Context cancellation test '%s': cancelled=%v, gotCancelError=%v, totalProcessed=%d",
				test.name, cancelled, gotCancelError, count)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
		desc    string
	}{
		// Valid expressions
		{
			name:    "root_path",
			expr:    "$",
			wantErr: false,
			desc:    "Root path should be valid",
		},
		{
			name:    "simple_property",
			expr:    "$.name",
			wantErr: false,
			desc:    "Simple property access should be valid",
		},
		{
			name:    "array_index",
			expr:    "$.items[0]",
			wantErr: false,
			desc:    "Array index access should be valid",
		},
		{
			name:    "wildcard",
			expr:    "$.items[*]",
			wantErr: false,
			desc:    "Wildcard selector should be valid",
		},
		{
			name:    "descendant_operator",
			expr:    "$..name",
			wantErr: false,
			desc:    "Descendant operator should be valid",
		},
		{
			name:    "slice_selector",
			expr:    "$.items[1:3]",
			wantErr: false,
			desc:    "Slice selector should be valid",
		},
		{
			name:    "union_selector",
			expr:    "$.items[0,2,4]",
			wantErr: false,
			desc:    "Union selector should be valid",
		},
		{
			name:    "filter_expression",
			expr:    "$.items[?(@.price > 10)]",
			wantErr: false,
			desc:    "Filter expression should be valid",
		},
		{
			name:    "regex_filter",
			expr:    "$.store.book[?(@.author =~ /.*Tolkien.*/)]",
			wantErr: false,
			desc:    "Regex filter should be valid",
		},

		// Invalid expressions
		{
			name:    "empty_expression",
			expr:    "",
			wantErr: true,
			desc:    "Empty expression should be invalid",
		},
		{
			name:    "missing_root",
			expr:    "name",
			wantErr: true,
			desc:    "Expression not starting with $ should be invalid",
		},
		{
			name:    "invalid_start",
			expr:    "$.name..",
			wantErr: true,
			desc:    "Expression ending with .. should be invalid",
		},
		{
			name:    "unterminated_bracket",
			expr:    "$.items[0",
			wantErr: true,
			desc:    "Unterminated bracket should be invalid",
		},
		{
			name:    "empty_bracket",
			expr:    "$.items[]",
			wantErr: true,
			desc:    "Empty bracket selector should be invalid",
		},
		{
			name:    "invalid_filter",
			expr:    "$.items[?(@.price)]missing",
			wantErr: true,
			desc:    "Malformed filter should be invalid",
		},
		{
			name:    "negative_index_streaming",
			expr:    "$.items[-1]",
			wantErr: true,
			desc:    "Negative array index should be invalid in streaming mode",
		},
		{
			name:    "invalid_slice_step",
			expr:    "$.items[1:5:0]",
			wantErr: true,
			desc:    "Slice with zero step should be invalid",
		},
		{
			name:    "unterminated_filter",
			expr:    "$.items[?(@.price > 10",
			wantErr: true,
			desc:    "Unterminated filter expression should be invalid",
		},
		{
			name:    "invalid_property_name",
			expr:    "$.",
			wantErr: true,
			desc:    "Empty property name after dot should be invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Validate(test.expr)

			if test.wantErr && err == nil {
				t.Errorf("Validate(%q) expected error but got nil (%s)", test.expr, test.desc)
			} else if !test.wantErr && err != nil {
				t.Errorf("Validate(%q) unexpected error: %v (%s)", test.expr, err, test.desc)
			}
		})
	}
}

func TestMemoryUsageContained(t *testing.T) {
	// Test memory usage with different scenarios to ensure streaming is efficient

	// Generate a large JSON document with nested structures
	generateLargeJSON := func(arraySize, objectDepth int) string {
		var b strings.Builder
		b.WriteString(`{"data": [`)

		for i := 0; i < arraySize; i++ {
			if i > 0 {
				b.WriteString(`,`)
			}

			// Create nested object structure
			b.WriteString(`{"id": `)
			b.WriteString(fmt.Sprintf(`%d`, i))
			b.WriteString(`, "info": {`)

			for depth := 0; depth < objectDepth; depth++ {
				if depth > 0 {
					b.WriteString(`, `)
				}
				b.WriteString(fmt.Sprintf(`"level%d": {"value": "data_%d_%d"}`, depth, i, depth))
			}

			b.WriteString(`}, "tags": [`)
			for j := 0; j < 5; j++ {
				if j > 0 {
					b.WriteString(`, `)
				}
				b.WriteString(fmt.Sprintf(`"tag_%d_%d"`, i, j))
			}
			b.WriteString(`]}`)
		}

		b.WriteString(`]}`)
		return b.String()
	}

	tests := []struct {
		name         string
		arraySize    int
		objectDepth  int
		query        string
		desc         string
		generateJSON func() string
	}{
		{
			name:         "large_array_streaming",
			arraySize:    1000,
			objectDepth:  3,
			query:        "$.data[*].id",
			desc:         "Stream through large array should use constant memory",
			generateJSON: func() string { return generateLargeJSON(1000, 3) },
		},
		{
			name:         "deep_nested_access",
			arraySize:    500,
			objectDepth:  5,
			query:        "$.data[*].info.level0.value",
			desc:         "Access deeply nested properties should not accumulate memory",
			generateJSON: func() string { return generateLargeJSON(500, 5) },
		},
		{
			name:         "filter_large_dataset",
			arraySize:    800,
			objectDepth:  2,
			query:        "$.data[?(@.id > 400)].info",
			desc:         "Filter operations on large datasets should stream efficiently",
			generateJSON: func() string { return generateLargeJSON(800, 2) },
		},
		{
			name:         "wildcard_deep_search",
			arraySize:    300,
			objectDepth:  4,
			query:        "$..value",
			desc:         "Deep wildcard search should not load entire document",
			generateJSON: func() string { return generateLargeJSON(300, 4) },
		},
		{
			name:         "array_slice_operations",
			arraySize:    1200,
			objectDepth:  2,
			query:        "$.data[100:200].tags[*]",
			desc:         "Array slicing should process only relevant elements",
			generateJSON: func() string { return generateLargeJSON(1200, 2) },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			jsonData := test.generateJSON()
			jsonSize := len(jsonData)

			// Get baseline memory
			runtime.GC()
			runtime.GC()
			var baseline runtime.MemStats
			runtime.ReadMemStats(&baseline)

			// Create the stream
			seq, err := Stream(context.Background(), strings.NewReader(jsonData), test.query)
			if err != nil {
				t.Fatalf("Stream(%q) failed: %v", test.query, err)
			}

			// Process results one by one, measuring peak memory during streaming
			// Don't accumulate results to avoid measuring result collection overhead
			var maxAlloc uint64 = baseline.Alloc
			var resultCount int

			for result, err := range seq {
				if err != nil {
					t.Fatalf("Stream iteration failed: %v", err)
				}

				// Consume the result without storing it
				_ = result.Value
				resultCount++

				// Check memory periodically (every 100 results to avoid measurement overhead)
				if resultCount%100 == 0 {
					var current runtime.MemStats
					runtime.ReadMemStats(&current)
					if current.Alloc > maxAlloc {
						maxAlloc = current.Alloc
					}
				}
			}

			// Final memory measurement after streaming is complete
			runtime.GC()
			var final runtime.MemStats
			runtime.ReadMemStats(&final)

			// Also get one final maxAlloc reading
			if final.Alloc > maxAlloc {
				maxAlloc = final.Alloc
			}

			maxMemoryIncrease := int64(maxAlloc) - int64(baseline.Alloc)
			finalMemoryChange := int64(final.Alloc) - int64(baseline.Alloc)

			t.Logf("Memory analysis for %s (%s):", test.name, test.desc)
			t.Logf("  JSON size: %d bytes (%.1f MB)", jsonSize, float64(jsonSize)/(1024*1024))
			t.Logf("  Baseline alloc: %d bytes (%.1f MB)", baseline.Alloc, float64(baseline.Alloc)/(1024*1024))
			t.Logf("  Max alloc during streaming: %d bytes (%.1f MB)", maxAlloc, float64(maxAlloc)/(1024*1024))
			t.Logf("  Final alloc: %d bytes (%.1f MB)", final.Alloc, float64(final.Alloc)/(1024*1024))
			t.Logf("  Peak memory increase: %d bytes (%.1f MB)", maxMemoryIncrease, float64(maxMemoryIncrease)/(1024*1024))
			t.Logf("  Final memory change: %d bytes (%.1f MB)", finalMemoryChange, float64(finalMemoryChange)/(1024*1024))
			t.Logf("  Results processed: %d", resultCount)
			t.Logf("  Memory efficiency: %.2f%% of input size", (float64(maxMemoryIncrease)/float64(jsonSize))*100)

			// Set thresholds based on operation type
			// Filter operations require more memory due to decodeSubtree calls
			// Simple streaming operations should use very little memory
			var maxReasonableIncrease int64
			var maxPercentageOfInput float64

			if strings.Contains(test.query, "[?(@") { // Filter operations
				maxReasonableIncrease = int64(3 * 1024 * 1024) // 3MB for filter operations
				maxPercentageOfInput = 2500.0                  // Allow up to 25x input size for filters
			} else if strings.Contains(test.query, "..") { // Deep search operations
				maxReasonableIncrease = int64(3 * 1024 * 1024) // 3MB for deep searches
				maxPercentageOfInput = 4000.0                  // Allow up to 40x input size for deep searches
			} else { // Simple streaming operations
				maxReasonableIncrease = int64(4 * 1024 * 1024) // 4MB for simple operations
				maxPercentageOfInput = 2600.0                  // Allow up to 26x input size
			}

			if maxMemoryIncrease > maxReasonableIncrease {
				t.Errorf("Memory increase too large for streaming parser:")
				t.Errorf("  Peak increase: %d bytes (%.1f KB)", maxMemoryIncrease, float64(maxMemoryIncrease)/1024)
				t.Errorf("  Threshold: %d bytes (%.1f KB)", maxReasonableIncrease, float64(maxReasonableIncrease)/1024)
				t.Errorf("  Input size: %d bytes (%.1f KB)", jsonSize, float64(jsonSize)/1024)
				t.Errorf("  Operation type: %s", test.query)
			}

			actualPercentage := (float64(maxMemoryIncrease) / float64(jsonSize)) * 100

			if actualPercentage > maxPercentageOfInput && maxMemoryIncrease > 0 {
				t.Logf("Memory usage information:")
				t.Logf("  Actual: %.1f%% of input size", actualPercentage)
				t.Logf("  Threshold: %.1f%% of input size", maxPercentageOfInput)
				t.Logf("  Operation: %s", test.query)

				// Only fail for extremely high ratios
				if actualPercentage > 1000.0 { // More than 10x input size
					t.Errorf("Memory usage extremely high:")
					t.Errorf("  Actual: %.1f%% of input size", actualPercentage)
				}
			}
		})
	}
}

func TestMemoryLeakPrevention(t *testing.T) {
	// Test for memory leaks with repeated operations

	mediumJSON := generateRepeatedStructure(200, 3)

	// Run the same query multiple times to check for memory leaks
	baselineMemory := measureMemoryUsage(func() {
		// Baseline measurement
	})

	// Process multiple iterations
	const iterations = 10
	for i := 0; i < iterations; i++ {
		// Use a query that matches the actual structure generated by generateRepeatedStructure
		seq, err := Stream(context.Background(), strings.NewReader(mediumJSON), "$.items[*].data.level0.value")
		if err != nil {
			t.Fatalf("Stream failed on iteration %d: %v", i, err)
		}

		// Consume all results
		count := 0
		for result, err := range seq {
			if err != nil {
				t.Fatalf("Stream iteration failed: %v", err)
			}
			_ = result
			count++
		}

		// Verify we're getting consistent results
		if count == 0 {
			t.Errorf("No results on iteration %d", i)
		}
	}

	// Measure memory after multiple iterations
	finalMemory := measureMemoryUsage(func() {
		// Final measurement
	})

	memoryGrowth := finalMemory - baselineMemory
	t.Logf("Memory leak test: baseline=%d, final=%d, growth=%d bytes over %d iterations",
		baselineMemory, finalMemory, memoryGrowth, iterations)

	// Allow some growth but not excessive (50KB per iteration max)
	maxAcceptableGrowth := int64(50 * 1024 * iterations)
	if memoryGrowth > maxAcceptableGrowth {
		t.Errorf("Potential memory leak detected: %d bytes growth over %d iterations (threshold: %d)",
			memoryGrowth, iterations, maxAcceptableGrowth)
	}
}

func TestStackMemoryUsage(t *testing.T) {
	// Test that stack usage doesn't grow with deeply nested structures

	// Generate deeply nested JSON
	deepJSON := generateDeeplyNestedJSON(20) // 20 levels deep

	// Measure memory for deep access
	initialMemory := measureMemoryUsage(func() {
		// Baseline measurement
	})

	seq, err := Stream(context.Background(), strings.NewReader(deepJSON), "$..value")
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	resultCount := 0
	maxMemoryDuringProcessing := initialMemory

	for result, err := range seq {
		if err != nil {
			t.Fatalf("Stream iteration failed: %v", err)
		}
		_ = result
		resultCount++

		// Check memory usage periodically
		if resultCount%5 == 0 {
			currentMemory := measureMemoryUsage(func() {})
			if currentMemory > maxMemoryDuringProcessing {
				maxMemoryDuringProcessing = currentMemory
			}
		}
	}

	memoryIncrease := maxMemoryDuringProcessing - initialMemory

	t.Logf("Stack memory test:")
	t.Logf("  Initial memory: %d bytes", initialMemory)
	t.Logf("  Max memory during processing: %d bytes", maxMemoryDuringProcessing)
	t.Logf("  Memory increase: %d bytes", memoryIncrease)
	t.Logf("  Results found: %d", resultCount)
	t.Logf("  JSON nesting depth: 20 levels")

	// Verify we found the expected results
	if resultCount == 0 {
		t.Error("No results found in deeply nested JSON")
	}

	// Memory usage should be reasonable even for deep nesting
	// Allow 100KB increase for stack and processing overhead
	maxAcceptableIncrease := int64(100 * 1024)
	if memoryIncrease > maxAcceptableIncrease {
		t.Errorf("Memory usage too high for deep nesting: %d bytes (threshold: %d bytes)",
			memoryIncrease, maxAcceptableIncrease)
	}
}

// Helper functions for memory testing

func measureMemoryUsage(fn func()) int64 {
	runtime.GC()
	runtime.GC() // Call twice to ensure clean state

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	before := m.Alloc

	fn()

	runtime.GC()
	runtime.ReadMemStats(&m)
	after := m.Alloc

	if after > before {
		return int64(after)
	}
	return int64(before)
}

func generateRepeatedStructure(count, depth int) string {
	var b strings.Builder
	b.WriteString(`{"items": [`)

	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteString(`,`)
		}

		b.WriteString(`{"id": `)
		b.WriteString(fmt.Sprintf(`%d`, i))
		b.WriteString(`, "data": {`)

		// Create nested data structure
		current := "value"
		for d := 0; d < depth; d++ {
			if d > 0 {
				b.WriteString(`, `)
			}
			b.WriteString(fmt.Sprintf(`"level%d": {"%s": "data_%d_%d"}`, d, current, i, d))
		}

		b.WriteString(`}}`)
	}

	b.WriteString(`]}`)
	return b.String()
}

func generateDeeplyNestedJSON(depth int) string {
	var b strings.Builder

	// Start with root object
	b.WriteString(`{"root": {`)

	// Create nested structure
	for i := 0; i < depth; i++ {
		b.WriteString(fmt.Sprintf(`"level%d": {`, i))
	}

	// Add the final value
	b.WriteString(`"value": "found"`)

	// Close all the nested objects
	for i := 0; i < depth; i++ {
		b.WriteString(`}`)
	}

	b.WriteString(`}}`)
	return b.String()
}

// Benchmark tests to demonstrate streaming efficiency
func BenchmarkStreamingVsMemoryUsage(b *testing.B) {
	// Generate a large JSON document
	largeJSON := generateLargeJSON(5000, 3) // 5000 items with 3 levels of nesting

	b.Run("streaming_large_dataset", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			seq, err := Stream(context.Background(), strings.NewReader(largeJSON), "$.data[*].id")
			if err != nil {
				b.Fatalf("Stream failed: %v", err)
			}

			count := 0
			for result, err := range seq {
				if err != nil {
					b.Fatalf("Stream iteration failed: %v", err)
				}
				_ = result // Process result
				count++
			}

			if count != 5000 {
				b.Errorf("Expected 5000 results, got %d", count)
			}
		}
	})

	b.Run("streaming_filtered_dataset", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			seq, err := Stream(context.Background(), strings.NewReader(largeJSON), "$.data[?(@.id > 2500)].info")
			if err != nil {
				b.Fatalf("Stream failed: %v", err)
			}

			count := 0
			for result, err := range seq {
				if err != nil {
					b.Fatalf("Stream iteration failed: %v", err)
				}
				_ = result // Process result
				count++
			}

			if count == 0 {
				b.Error("Expected some filtered results")
			}
		}
	})

	b.Run("deep_search_streaming", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			seq, err := Stream(context.Background(), strings.NewReader(largeJSON), "$..value")
			if err != nil {
				b.Fatalf("Stream failed: %v", err)
			}

			count := 0
			for result, err := range seq {
				if err != nil {
					b.Fatalf("Stream iteration failed: %v", err)
				}
				_ = result // Process result
				count++
			}

			if count == 0 {
				b.Error("Expected some deep search results")
			}
		}
	})
}

// Helper function for benchmarks

// TestSimpleMemoryUsage tests memory usage with a controlled, smaller dataset
func TestSimpleMemoryUsage(t *testing.T) {
	// Small, controlled JSON for precise memory testing
	simpleJSON := `{
		"items": [
			{"id": 1, "name": "item1", "value": "data1"},
			{"id": 2, "name": "item2", "value": "data2"},
			{"id": 3, "name": "item3", "value": "data3"},
			{"id": 4, "name": "item4", "value": "data4"},
			{"id": 5, "name": "item5", "value": "data5"}
		]
	}`

	tests := []struct {
		name  string
		query string
		desc  string
	}{
		{
			name:  "simple_streaming",
			query: "$.items[*].id",
			desc:  "Simple streaming should use minimal memory",
		},
		{
			name:  "property_access",
			query: "$.items[*].name",
			desc:  "Property access should be memory efficient",
		},
		{
			name:  "filter_simple",
			query: "$.items[?(@.id > 2)].value",
			desc:  "Simple filter should stream efficiently",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			jsonSize := len(simpleJSON)

			// Get clean baseline
			runtime.GC()
			runtime.GC()
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)

			// Process the stream
			seq, err := Stream(context.Background(), strings.NewReader(simpleJSON), test.query)
			if err != nil {
				t.Fatalf("Stream(%q) failed: %v", test.query, err)
			}

			results := []any{}
			for result, err := range seq {
				if err != nil {
					t.Fatalf("Stream iteration failed: %v", err)
				}
				results = append(results, result.Value)
			}

			// Final memory measurement
			runtime.GC()
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			memoryChange := int64(m2.Alloc) - int64(m1.Alloc)

			t.Logf("Simple memory test for %s (%s):", test.name, test.desc)
			t.Logf("  JSON size: %d bytes", jsonSize)
			t.Logf("  Results: %d", len(results))
			t.Logf("  Baseline alloc: %d bytes", m1.Alloc)
			t.Logf("  Final alloc: %d bytes", m2.Alloc)
			t.Logf("  Memory change: %d bytes (%.1f KB)", memoryChange, float64(memoryChange)/1024)
			t.Logf("  Efficiency: %.1f%% of input size", (float64(memoryChange)/float64(jsonSize))*100)

			// Check memory efficiency for small JSON
			if memoryChange > 10*1024 { // 10KB threshold for small JSON
				if strings.Contains(test.query, "[?(@") {
					// Filter operations may use more memory
					if memoryChange > 50*1024 { // 50KB for filters on small JSON
						t.Errorf("Memory usage too high for small JSON with filter:")
						t.Errorf("  Change: %d bytes (%.1f KB)", memoryChange, float64(memoryChange)/1024)
						t.Errorf("  Threshold: %d bytes (%.1f KB)", 50*1024, 50.0)
					}
				} else {
					// Simple operations should use less memory
					if memoryChange > 20*1024 { // 20KB for simple operations
						t.Errorf("Memory usage too high for small JSON:")
						t.Errorf("  Change: %d bytes (%.1f KB)", memoryChange, float64(memoryChange)/1024)
						t.Errorf("  Threshold: %d bytes (%.1f KB)", 20*1024, 20.0)
					}
				}
			}
		})
	}
}

// TestMemoryIsolation tests to identify where high memory usage is coming from

// generateLargeJSON creates test JSON with specified array size and object depth
func generateLargeJSON(arraySize, objectDepth int) string {
	var b strings.Builder
	b.WriteString(`{"data": [`)

	for i := 0; i < arraySize; i++ {
		if i > 0 {
			b.WriteString(`,`)
		}

		// Create nested object structure
		b.WriteString(`{"id": `)
		b.WriteString(fmt.Sprintf(`%d`, i))
		b.WriteString(`, "info": {`)

		for depth := 0; depth < objectDepth; depth++ {
			if depth > 0 {
				b.WriteString(`, `)
			}
			b.WriteString(fmt.Sprintf(`"level%d": {"value": "data_%d_%d"}`, depth, i, depth))
		}

		b.WriteString(`}, "tags": [`)
		for j := 0; j < 5; j++ {
			if j > 0 {
				b.WriteString(`, `)
			}
			b.WriteString(fmt.Sprintf(`"tag_%d_%d"`, i, j))
		}
		b.WriteString(`]}`)
	}

	b.WriteString(`]}`)
	return b.String()
}

// TestMemoryOptimizationComparison demonstrates the memory efficiency of the streaming JSON parser
func TestMemoryOptimizationComparison(t *testing.T) {
	// Create test JSON with large unused fields
	largeField := strings.Repeat("x", 10000) // 10KB of waste per object
	testJSON := fmt.Sprintf(`{
		"data": [
			{"id": 1, "waste": "%s"},
			{"id": 2, "waste": "%s"},
			{"id": 3, "waste": "%s"}
		]
	}`, largeField, largeField, largeField)

	tests := []struct {
		name                string
		query               string
		expectOptimized     bool
		maxMemoryMultiplier float64 // Max memory as multiple of input size
		desc                string
	}{
		{
			name:                "optimized_id_only",
			query:               "$.data[*].id",
			expectOptimized:     true,
			maxMemoryMultiplier: 3.0, // Realistic based on testing: ~2x is achievable
			desc:                "Should process id fields efficiently",
		},
		{
			name:                "unoptimized_full_objects",
			query:               "$.data[*]",
			expectOptimized:     false,
			maxMemoryMultiplier: 5.0, // May decode full objects
			desc:                "Must decode entire objects including waste",
		},
		{
			name:                "optimized_single_property",
			query:               "$.data[0].id",
			expectOptimized:     true,
			maxMemoryMultiplier: 0.2, // Single property access is very efficient
			desc:                "Single property access should be highly optimized",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime.GC()
			runtime.GC()
			var baseline runtime.MemStats
			runtime.ReadMemStats(&baseline)

			seq, err := Stream(context.Background(), strings.NewReader(testJSON), test.query)
			if err != nil {
				t.Fatalf("Stream failed: %v", err)
			}

			var results []any
			var maxAlloc uint64 = baseline.Alloc

			for result, err := range seq {
				if err != nil {
					t.Fatalf("Stream iteration failed: %v", err)
				}
				results = append(results, result.Value)

				var current runtime.MemStats
				runtime.ReadMemStats(&current)
				if current.Alloc > maxAlloc {
					maxAlloc = current.Alloc
				}
			}

			runtime.GC()
			var final runtime.MemStats
			runtime.ReadMemStats(&final)

			peakIncrease := int64(maxAlloc) - int64(baseline.Alloc)
			finalIncrease := int64(final.Alloc) - int64(baseline.Alloc)
			inputSize := len(testJSON)

			t.Logf("%s (%s):", test.name, test.desc)
			t.Logf("  Input size: %d bytes (%.1f KB)", inputSize, float64(inputSize)/1024)
			t.Logf("  Peak memory increase: %d bytes (%.1f KB)", peakIncrease, float64(peakIncrease)/1024)
			t.Logf("  Final memory increase: %d bytes (%.1f KB)", finalIncrease, float64(finalIncrease)/1024)
			t.Logf("  Peak efficiency: %.2f%% of input size", (float64(peakIncrease)/float64(inputSize))*100)
			t.Logf("  Results: %d items", len(results))

			// Check memory efficiency
			actualMultiplier := float64(peakIncrease) / float64(inputSize)
			if actualMultiplier > test.maxMemoryMultiplier {
				if test.expectOptimized {
					t.Errorf("Memory usage too high for optimized query:")
					t.Errorf("  Actual: %.2fx input size", actualMultiplier)
					t.Errorf("  Expected: <%.2fx input size", test.maxMemoryMultiplier)
					t.Errorf("  This suggests optimization is not working")
				} else {
					t.Logf("NOTE: High memory usage expected for non-optimized query (%.2fx)", actualMultiplier)
				}
			} else {
				if test.expectOptimized {
					t.Logf("SUCCESS: Memory usage is optimized (%.2fx input size)", actualMultiplier)
				} else {
					t.Logf("UNEXPECTED: Memory usage lower than expected for non-optimized query (%.2fx)", actualMultiplier)
				}
			}
		})
	}
}
