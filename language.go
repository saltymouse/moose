package main

import "strings"

var isoLanguages = map[string]string{
	"ja": "Japanese",
	"ko": "Korean",
	"zh": "Chinese",
	"en": "English",
	"hi": "Hindi",
	"fr": "French",
	"de": "German",
	"es": "Spanish",
	"it": "Italian",
	"ru": "Russian",
	"pt": "Portuguese",
	"ar": "Arabic",
	"th": "Thai",
	"tr": "Turkish",
	"sv": "Swedish",
	"nb": "Norwegian",
	"da": "Danish",
	"fi": "Finnish",
	"pl": "Polish",
	"nl": "Dutch",
	"id": "Indonesian",
	"cs": "Czech",
	"el": "Greek",
	"he": "Hebrew",
	"hu": "Hungarian",
	"uk": "Ukrainian",
	"vi": "Vietnamese",
	"ro": "Romanian",
}

// LanguageTag maps an ISO 639-1 code to "Language: <DisplayName>".
// Unknown codes are uppercased: "bn" → "Language: BN".
func LanguageTag(code string) string {
	if name, ok := isoLanguages[code]; ok {
		return "Language: " + name
	}
	return "Language: " + strings.ToUpper(code)
}
