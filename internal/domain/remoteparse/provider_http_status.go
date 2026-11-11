package remoteparse

import "strings"

// ProviderHTTPStatus returns the HTTP error status a provider CLI diagnostic
// reports with certainty, or 0 when it reports none or more than one.
//
// A status is a standalone three-digit number from 400 to 599, with no digit
// directly before or after it, that appears either as the prefix "HTTP NNN:"
// (the gh wording, for example "HTTP 401: Bad credentials") or as
// "NNN " followed by that code's standard reason phrase (for example
// "ERROR 404 Not Found"). A bare number, a number with another phrase, or a
// number inside a longer token is not a status. Two different statuses in one
// diagnostic are ambiguous and yield 0.
func ProviderHTTPStatus(diagnostic string) int {
	found := 0
	for i := 0; i+3 <= len(diagnostic); i++ {
		code, ok := providerHTTPStatusAt(diagnostic, i)
		if !ok {
			continue
		}
		if found != 0 && found != code {
			return 0
		}
		found = code
	}
	return found
}

// providerHTTPStatusAt reads a status that starts at offset i of diagnostic.
func providerHTTPStatusAt(diagnostic string, i int) (int, bool) {
	if !isASCIIDigit(diagnostic[i]) || !isASCIIDigit(diagnostic[i+1]) || !isASCIIDigit(diagnostic[i+2]) {
		return 0, false
	}
	if i > 0 && isASCIIDigit(diagnostic[i-1]) {
		return 0, false
	}
	end := i + 3
	if end < len(diagnostic) && isASCIIDigit(diagnostic[end]) {
		return 0, false
	}
	code := int(diagnostic[i]-'0')*100 + int(diagnostic[i+1]-'0')*10 + int(diagnostic[i+2]-'0')
	if code < 400 || code > 599 {
		return 0, false
	}
	if strings.HasSuffix(diagnostic[:i], "HTTP ") && strings.HasPrefix(diagnostic[end:], ":") {
		return code, true
	}
	phrase, known := httpReasonPhrases[code]
	rest := diagnostic[end:]
	if !known || !strings.HasPrefix(rest, " "+phrase) {
		return 0, false
	}
	if after := rest[1+len(phrase):]; after != "" && isWordByte(after[0]) {
		return 0, false
	}
	return code, true
}

func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }

// isWordByte reports whether b continues a word: an ASCII letter or digit, or
// any byte of a non-ASCII UTF-8 sequence.
func isWordByte(b byte) bool {
	return isASCIIDigit(b) || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// httpReasonPhrases holds the standard 4xx and 5xx reason phrases exactly as
// Go's net/http.StatusText spells them. The domain layer cannot import
// net/http, so the table is repeated here and a test keeps the two in step.
var httpReasonPhrases = map[int]string{
	400: "Bad Request",
	401: "Unauthorized",
	402: "Payment Required",
	403: "Forbidden",
	404: "Not Found",
	405: "Method Not Allowed",
	406: "Not Acceptable",
	407: "Proxy Authentication Required",
	408: "Request Timeout",
	409: "Conflict",
	410: "Gone",
	411: "Length Required",
	412: "Precondition Failed",
	413: "Request Entity Too Large",
	414: "Request URI Too Long",
	415: "Unsupported Media Type",
	416: "Requested Range Not Satisfiable",
	417: "Expectation Failed",
	418: "I'm a teapot",
	421: "Misdirected Request",
	422: "Unprocessable Entity",
	423: "Locked",
	424: "Failed Dependency",
	425: "Too Early",
	426: "Upgrade Required",
	428: "Precondition Required",
	429: "Too Many Requests",
	431: "Request Header Fields Too Large",
	451: "Unavailable For Legal Reasons",
	500: "Internal Server Error",
	501: "Not Implemented",
	502: "Bad Gateway",
	503: "Service Unavailable",
	504: "Gateway Timeout",
	505: "HTTP Version Not Supported",
	506: "Variant Also Negotiates",
	507: "Insufficient Storage",
	508: "Loop Detected",
	510: "Not Extended",
	511: "Network Authentication Required",
}
