package diagnostics

import "testing"

func TestDefinitionForKnownCodes(t *testing.T) {
	t.Parallel()

	codes := []Code{
		CodeInvalidRequestShape,
		CodeAuthNotMapped,
		CodeBodyNotSupported,
		CodeTestNotMapped,
		CodeScriptLineUnmapped,
		CodeScriptExpressionNotSupported,
		CodeScriptJSONPathTranslationFailed,
		CodeQueryDuplicate,
		CodeTemplatePlaceholderUnsupported,
		CodeOutputExists,
	}

	for _, code := range codes {
		definition := DefinitionFor(code)
		if definition.Code != code {
			t.Fatalf("definition.Code = %q, want %q", definition.Code, code)
		}
		if definition.DefaultStage == "" {
			t.Fatalf("definition.DefaultStage is empty for code %q", code)
		}
		if definition.DefaultSeverity == "" {
			t.Fatalf("definition.DefaultSeverity is empty for code %q", code)
		}
	}
}
