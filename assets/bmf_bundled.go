//go:build bundled

package assets

import _ "embed"

//go:embed regkassen-verification-1.1.1.zip
var bmfZip []byte

func init() { BMFEngine = bmfZip }
