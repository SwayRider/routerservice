package types

type Language string

const (
	Bulgarian				Language = "bg-BG"
	BulgarianAlias			Language = "bg"
	Catalan					Language = "ca-ES"
	CatalanAlias			Language = "ca"
	Czech					Language = "cs-CZ"
	CzechAlias				Language = "cs"
	Danish					Language = "da-DK"
	DanishAlias				Language = "da"
	German					Language = "de-DE"
	GermanAlias				Language = "de"
	Greek					Language = "el-GR"
	GreekAlias				Language = "el"
	EnglishGB				Language = "en-GB"
	EnglishUSPirate			Language = "en-US-x-pirate"
	EnglishUSPirateAlias	Language = "en-x-pirate"
	EnglishUS				Language = "en-US"
	EnglishUSAlias			Language = "en"
	English					Language = "en-US"
	EnglishAlias			Language = "en"
	Spanish					Language = "es-ES"
	SpanishAlias			Language = "es"
	Estonian				Language = "et-EE"
	EstonianAlias			Language = "et"
	Finnish					Language = "fi-FI"
	FinnishAlias			Language = "fi"
	French					Language = "fr-FR"
	FrenchAlias				Language = "fr"
	Hindi					Language = "hi-IN"
	HindiAlias				Language = "hi"
	Hungarian				Language = "hu-HU"
	HungarianAlias			Language = "hu"
	Italian					Language = "it-IT"
	ItalianAlias			Language = "it"
	Japanese				Language = "ja-JP"
	JapaneseAlias			Language = "ja"
	Bokmal					Language = "nb-NO"
	BokmalAlias				Language = "nb"
	Dutch					Language = "nl-NL"
	DutchAlias				Language = "nl"
	Polish					Language = "pl-PL"
	PolishAlias				Language = "pl"
	PortugeseBR				Language = "pt-BR"
	Portugese				Language = "pt-PT"
	PortugeseAlias			Language = "pt"
	Romanian				Language = "ro-RO"
	RomanianAlias			Language = "ro"
	Russian					Language = "ru-RU"
	RussianAlias			Language = "ru"
	Slovak					Language = "sk-SK"
	SlovakAlias				Language = "sk"
	Slovenian				Language = "sl-SI"
	SlovenianAlias			Language = "sl"
	Swedish					Language = "sv-SE"
	SwedishAlias			Language = "sv"
	Turkish					Language = "tr-TR"
	TurkishAlias			Language = "tr"
	Ukrainian				Language = "uk-UA"
	UkrainianAlias			Language = "uk"
)

func (l Language) ToString() string {
	tmp := l
	switch l {
	case BulgarianAlias:
		tmp = Bulgarian
	case CatalanAlias:
		tmp = Catalan
	case CzechAlias:
		tmp = Czech
	case DanishAlias:
		tmp = Danish
	case GermanAlias:
		tmp = German
	case GreekAlias:
		tmp = Greek
	case EnglishUSPirateAlias:
		tmp = EnglishUSPirate
	case EnglishAlias:
		tmp = English
	case SpanishAlias:
		tmp = Spanish
	case EstonianAlias:
		tmp = Estonian
	case FinnishAlias:
		tmp = Finnish
	case FrenchAlias:
		tmp = French
	case HindiAlias:
		tmp = Hindi
	case HungarianAlias:
		tmp = Hungarian
	case ItalianAlias:
		tmp = Italian
	case JapaneseAlias:
		tmp = Japanese
	case BokmalAlias:
		tmp = Bokmal
	case DutchAlias:
		tmp = Dutch
	case PolishAlias:
		tmp = Polish
	case PortugeseAlias:
		tmp = Portugese
	case RomanianAlias:
		tmp = Romanian
	case RussianAlias:
		tmp = Russian
	case SlovakAlias:
		tmp = Slovak
	case SlovenianAlias:
		tmp = Slovenian
	case SwedishAlias:
		tmp = Swedish
	case TurkishAlias:
		tmp = Turkish
	case UkrainianAlias:
		tmp = Ukrainian
	}
	return string(tmp)
}
