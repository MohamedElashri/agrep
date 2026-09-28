package arabic

// foldRasm maps the Arabic letters whose consonantal skeleton differs only by
// i'jam to one representative. Language-specific letters are deliberately not
// included: Urdu retroflexes, Pashto letters, and Kurdish or Uyghur vowels keep
// their distinctions even when Rasm is enabled.
func foldRasm(r rune) rune {
	switch r {
	case 'ب', 'ت', 'ث', 'ن', 'ي':
		return 'ٮ'
	case 'ج', 'ح', 'خ':
		return 'ح'
	case 'د', 'ذ':
		return 'د'
	case 'ر', 'ز':
		return 'ر'
	case 'س', 'ش':
		return 'س'
	case 'ص', 'ض':
		return 'ص'
	case 'ط', 'ظ':
		return 'ط'
	case 'ع', 'غ':
		return 'ع'
	case 'ف', 'ق':
		return 'ٯ'
	default:
		return r
	}
}
