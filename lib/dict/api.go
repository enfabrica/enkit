package dict

var Endpoint = "https://api.dictionaryapi.dev/api/<--version-->/entries/<--language_code-->/<--word-->"

type Definition struct {
	Definition string   `json:"definition"`
	Example    string   `json:"example"`
	Synonims   []string `json:"synonims"`
}

type Meaning struct {
	PartOfSpeech string       `json:"partOfSpeech"`
	Defintions   []Definition `json:"definitions"`
}

type Entry struct {
	Title      string `json:"title"`
	Message    string `json:"message"`
	Resolution string `json:"resolution"`

	Word     string    `json:"word"`
	Phonetic string    `json:"phonetic"`
	Origin   string    `json:"origin"`
	Meanings []Meaning `json:"meanings"`
}
