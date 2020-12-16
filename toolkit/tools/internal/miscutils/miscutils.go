// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package miscutils

import "crypto/rand"

// RandomString generates a random string of the length specified
// using the provided legalCharacters.  crypto.rand is more secure
// than math.rand and does not need to be seeded.
func RandomString(length int, legalCharacters string) (output string, err error) {
	b := make([]byte, length)
	_, err = rand.Read(b)
	if err != nil {
		return
	}

	count := byte(len(legalCharacters))
	for i := range b {
		idx := b[i] % count
		b[i] = legalCharacters[idx]
	}

	output = string(b)
	return
}
