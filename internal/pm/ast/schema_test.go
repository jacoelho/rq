package ast

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()

	input := `
{
  "info": {"name": "Sample", "schema": "v2"},
  "event": [
    {
      "listen": "test",
      "script": {"exec": ["tests[\"root status\"] = responseCode.code === 200;"]}
    }
  ],
  "item": [
    {
      "name": "Get user",
      "request": {
        "method": "GET",
        "url": "https://api.example.com/users/{{user_id}}"
      }
    },
    {
      "name": "Create user",
      "request": {
        "method": "POST",
        "url": {
          "raw": "https://api.example.com/users",
          "query": [{"key": "expand", "value": "true"}]
        }
      }
    },
    {
      "name": "Upload file",
      "request": {
        "method": "PUT",
        "url": "https://api.example.com/upload",
        "body": {
          "mode": "file",
          "file": {
            "src": "/tmp/payload.bin"
          }
        }
      }
    },
    {
      "name": "Health",
      "request": {
        "method": "GET",
        "url": {
          "protocol": "http",
          "host": ["localhost"],
          "port": "8080",
          "path": ["health"]
        }
      }
    }
  ]
}
`

	collection, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if collection.Info.Name != "Sample" {
		t.Fatalf("info.name = %q", collection.Info.Name)
	}
	if len(collection.Event) != 1 {
		t.Fatalf("root event length = %d", len(collection.Event))
	}
	if collection.Event[0].Listen != "test" {
		t.Fatalf("root event listen = %q", collection.Event[0].Listen)
	}
	if len(collection.Event[0].Script.Exec) != 1 || collection.Event[0].Script.Exec[0] != `tests["root status"] = responseCode.code === 200;` {
		t.Fatalf("unexpected root event script = %#v", collection.Event[0].Script.Exec)
	}

	if got := collection.Item[0].Request.URL.Raw; got != "https://api.example.com/users/{{user_id}}" {
		t.Fatalf("first URL raw = %q", got)
	}

	if got := collection.Item[1].Request.URL.Raw; got != "https://api.example.com/users" {
		t.Fatalf("second URL raw = %q", got)
	}
	if len(collection.Item[1].Request.URL.Query) != 1 {
		t.Fatalf("query length = %d", len(collection.Item[1].Request.URL.Query))
	}
	if collection.Item[2].Request.Body == nil || collection.Item[2].Request.Body.File == nil {
		t.Fatal("expected file body metadata")
	}
	if collection.Item[2].Request.Body.File.Src != "/tmp/payload.bin" {
		t.Fatalf("file src = %q", collection.Item[2].Request.Body.File.Src)
	}
	if collection.Item[3].Request.URL.Port != "8080" {
		t.Fatalf("health URL port = %q", collection.Item[3].Request.URL.Port)
	}
}

func TestRequestEffectiveURLPortFallbackAndPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("falls back to urlObject port", func(t *testing.T) {
		t.Parallel()

		request := Request{
			URL: URLValue{
				Protocol: "http",
				Host:     []string{"localhost"},
				Path:     []string{"health"},
			},
			URLObject: &URLObject{
				Port: "8080",
			},
		}

		resolved := request.EffectiveURL()
		if resolved.Port != "8080" {
			t.Fatalf("resolved port = %q", resolved.Port)
		}
	})

	t.Run("prefers primary URL port", func(t *testing.T) {
		t.Parallel()

		request := Request{
			URL: URLValue{
				Protocol: "http",
				Port:     "9090",
				Host:     []string{"localhost"},
				Path:     []string{"health"},
			},
			URLObject: &URLObject{
				Port: "8080",
			},
		}

		resolved := request.EffectiveURL()
		if resolved.Port != "9090" {
			t.Fatalf("resolved port = %q", resolved.Port)
		}
	})
}
