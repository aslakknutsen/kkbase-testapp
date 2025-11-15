package behavior

// parserFunc is a function that parses a behavior value and sets it on the Behavior struct
type parserFunc func(b *Behavior, value string) error

// parsers maps behavior keys to their parser functions
var parsers = make(map[string]parserFunc)

// registerParser registers a parser function for a given behavior key
func registerParser(key string, fn parserFunc) {
	parsers[key] = fn
}

// mergeField merges two optional behavior fields, with b2 taking precedence over b1
func mergeField[T any](b1, b2 *T) *T {
	if b2 != nil {
		return b2
	}
	return b1
}

