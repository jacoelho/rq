package diagnostics

// Code classifies migration limitations and skips.
type Code string

const (
	CodeInvalidRequestShape             Code = "invalid_request_shape"
	CodeAuthNotMapped                   Code = "auth_not_mapped"
	CodeBodyNotSupported                Code = "body_mode_not_supported"
	CodeTestNotMapped                   Code = "test_script_not_mapped"
	CodeScriptLineUnmapped              Code = "script_line_unmapped"
	CodeScriptExpressionNotSupported    Code = "script_expression_not_supported"
	CodeScriptJSONPathTranslationFailed Code = "script_jsonpath_translation_failed"
	CodeQueryDuplicate                  Code = "query_duplicate_key"
	CodeTemplatePlaceholderUnsupported  Code = "template_placeholder_unsupported"
	CodeOutputExists                    Code = "output_exists"
)

// Stage identifies the migration pipeline stage where a diagnostic was raised.
type Stage string

const (
	StageNormalize  Stage = "normalize"
	StageRequestMap Stage = "requestmap"
	StageLower      Stage = "lower"
	StageFiles      Stage = "files"
)

// Severity indicates diagnostic impact.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Definition is canonical metadata for one diagnostic code.
type Definition struct {
	Code            Code
	DefaultStage    Stage
	DefaultSeverity Severity
}

var definitions = map[Code]Definition{
	CodeInvalidRequestShape: {
		Code:            CodeInvalidRequestShape,
		DefaultStage:    StageRequestMap,
		DefaultSeverity: SeverityError,
	},
	CodeAuthNotMapped: {
		Code:            CodeAuthNotMapped,
		DefaultStage:    StageRequestMap,
		DefaultSeverity: SeverityWarning,
	},
	CodeBodyNotSupported: {
		Code:            CodeBodyNotSupported,
		DefaultStage:    StageRequestMap,
		DefaultSeverity: SeverityError,
	},
	CodeTestNotMapped: {
		Code:            CodeTestNotMapped,
		DefaultStage:    StageLower,
		DefaultSeverity: SeverityError,
	},
	CodeScriptLineUnmapped: {
		Code:            CodeScriptLineUnmapped,
		DefaultStage:    StageLower,
		DefaultSeverity: SeverityError,
	},
	CodeScriptExpressionNotSupported: {
		Code:            CodeScriptExpressionNotSupported,
		DefaultStage:    StageLower,
		DefaultSeverity: SeverityError,
	},
	CodeScriptJSONPathTranslationFailed: {
		Code:            CodeScriptJSONPathTranslationFailed,
		DefaultStage:    StageLower,
		DefaultSeverity: SeverityError,
	},
	CodeQueryDuplicate: {
		Code:            CodeQueryDuplicate,
		DefaultStage:    StageRequestMap,
		DefaultSeverity: SeverityWarning,
	},
	CodeTemplatePlaceholderUnsupported: {
		Code:            CodeTemplatePlaceholderUnsupported,
		DefaultStage:    StageRequestMap,
		DefaultSeverity: SeverityWarning,
	},
	CodeOutputExists: {
		Code:            CodeOutputExists,
		DefaultStage:    StageFiles,
		DefaultSeverity: SeverityWarning,
	},
}

// DefinitionFor resolves canonical metadata for a diagnostic code.
func DefinitionFor(code Code) Definition {
	if definition, ok := definitions[code]; ok {
		return definition
	}

	return Definition{
		Code:            code,
		DefaultStage:    StageRequestMap,
		DefaultSeverity: SeverityWarning,
	}
}

// Span identifies a source range.
type Span struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

// Issue is a single migration diagnostic.
type Issue struct {
	Code     Code     `json:"code"`
	Stage    Stage    `json:"stage,omitempty"`
	Path     string   `json:"path,omitempty"`
	Severity Severity `json:"severity,omitempty"`
	Message  string   `json:"message"`
	Span     *Span    `json:"span,omitempty"`
}
